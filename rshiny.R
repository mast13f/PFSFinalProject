library(shiny)
library(processx)
library(magick)

# -------------------- ADDED: limit upload size  ======================
# Allow max upload 5 MB (including headers)
options(shiny.maxRequestSize = 5 * 1024^2)  # === CHANGED/ADDED ===
# ---------------------------------------------------------------------------

# -------------------- Hardcoded configuration  --------------------
project_dir <- ".../go/src/PFSFinalProject"   # Root directory of the Go project
exec_cmd <- "./PFSFinalProject"                           # Executable to run under project_dir
default_timeout <- 300                                    # Timeout in seconds
# ----------------------------------------------------------------

# Ensure the app's www directory exists (for static resources)
app_www <- file.path(getwd(), "www")
if (!dir.exists(app_www)) dir.create(app_www, recursive = TRUE)

# Map app_www to URL prefix "gifs" so browser can access /gifs/<file>
addResourcePath("gifs", app_www)

ui <- fluidPage(
  titlePanel("Epidemic Simulation"),
  sidebarLayout(
    sidebarPanel(
      ## === CHANGED/ADDED ===
      # Allow user to upload config.txt
      fileInput("config_file", "Upload config.txt", accept = c(".txt"), multiple = FALSE),
      tags$hr(),
      ## === END CHANGED ===
      
      actionButton("run_sim", "Run Simulation", class = "btn-primary"),
      tags$hr(),
      h5("Log"),
      verbatimTextOutput("run_log", placeholder = TRUE),
      width = 3
    ),
    mainPanel(
      h3("Simulation Output (GIFs)"),
      tags$hr(),
      fluidRow(
        column(8,
               h4("GIF Preview"),
               uiOutput("gif_ui")
        ),
        column(4,
               h4("GIF Data"),
               tableOutput("gif_meta"),
               tags$hr(),
               uiOutput("download_ui")
        )
      ),
      tags$hr(),
      h4("Status and Colors"),
      tags$ul(
        # Priority: vaccinated and not dead
        tags$li(
          tags$span(style = "display:inline-block;width:18px;height:18px;background:rgb(64,128,64);border:1px solid #aaa;margin-right:8px;vertical-align:middle;"),
          tags$strong("Vaccinated")),
        tags$li(
          tags$span(style = "display:inline-block;width:18px;height:18px;background:rgb(128,255,0);border:1px solid #aaa;margin-right:8px;vertical-align:middle;"),
          tags$strong("Healthy")
        ),
        tags$li(
          tags$span(style = "display:inline-block;width:18px;height:18px;background:rgb(255,255,0);border:1px solid #aaa;margin-right:8px;vertical-align:middle;"),
          tags$strong("Susceptible")
        ),
        tags$li(
          tags$span(style = "display:inline-block;width:18px;height:18px;background:rgb(255,0,0);border:1px solid #aaa;margin-right:8px;vertical-align:middle;"),
          tags$strong("Infected")),
        tags$li(
          tags$span(style = "display:inline-block;width:18px;height:18px;background:rgb(0,128,255);border:1px solid #aaa;margin-right:8px;vertical-align:middle;"),
          tags$strong("Recovered")
        ),
        tags$li(
          tags$span(style = "display:inline-block;width:18px;height:18px;background:rgb(160,160,160);border:1px solid #aaa;margin-right:8px;vertical-align:middle;"),
          tags$strong("Dead")
        ),
        tags$hr()
      )
    )
  )
)

server <- function(input, output, session) {
  rv <- reactiveValues(log = "", latest_gifs = character(0), gif_infos = list())
  
  # Find the latest n .gif files under project_dir (sorted by modification time desc)
  find_latest_gifs <- function(dir, n = 2) {
    if (!dir.exists(dir)) return(character(0))
    files <- list.files(dir, pattern = "\\.gif$", ignore.case = TRUE, full.names = TRUE, recursive = TRUE)
    if (length(files) == 0) return(character(0))
    infos <- file.info(files)
    files[order(infos$mtime, decreasing = TRUE)][seq_len(min(n, length(files)))]
  }
  
  observeEvent(input$run_sim, {
    # Reset previous state
    rv$log <- ""
    rv$latest_gifs <- character(0)
    rv$gif_infos <- list()
    
    showModal(modalDialog(
      title = "Running",
      paste0("Will execute in directory ", project_dir, ": ", exec_cmd, "\nPlease wait for the program to finish or timeout..."),
      footer = NULL,
      easyClose = FALSE
    ))
    
    # Basic checks
    if (!dir.exists(project_dir)) {
      rv$log <- paste0("ERROR: Project directory not found: ", project_dir)
      removeModal()
      return()
    }
    
    # === CHANGED/ADDED ===
    # Handle uploaded config file 
    config_arg <- character(0)   # empty by default
    uploaded_saved_path <- NULL  # record saved path in project_dir for cleanup
    if (!is.null(input$config_file) && !is.null(input$config_file$datapath)) {
      # basic sanity checks: extension check, content contains '='
      up <- input$config_file
      orig_name <- up$name
      # Allow .txt (or no extension — still try but warn)
      ext <- tolower(tools::file_ext(orig_name))
      if (ext == "") {
        # no extension: allow but warn
        rv$log <- paste0(rv$log, "Note: uploaded file has no extension; will be treated as text.\n")
      } else if (ext != "txt") {
        rv$log <- paste0(rv$log, "ERROR: uploaded file extension is not .txt; please upload config.txt.\n")
        removeModal()
        return()
      }
      # Read content and perform a simple validation
      content_ok <- FALSE
      try({
        lines <- readLines(up$datapath, warn = FALSE)
        # require at least one line containing "="
        if (length(lines) > 0 && any(grepl("=", lines))) content_ok <- TRUE
      }, silent = TRUE)
      if (!content_ok) {
        rv$log <- paste0(rv$log, "ERROR: uploaded config file format invalid (expected key=value lines with at least one '=').\n")
        removeModal()
        return()
      }
      # Copy uploaded file into project_dir so the Go executable can access it (timestamped safe name)
      ts <- format(Sys.time(), "%Y%m%d%H%M%S")
      safe_base <- gsub("[^A-Za-z0-9_.-]", "_", tools::file_path_sans_ext(orig_name))
      dest_name <- paste0("config_upload_", ts, "_", safe_base, ".txt")
      dest_path <- file.path(project_dir, dest_name)
      okcopy <- tryCatch({
        file.copy(up$datapath, dest_path, overwrite = TRUE)
      }, error = function(e) FALSE)
      if (!okcopy) {
        rv$log <- paste0(rv$log, "ERROR: failed to copy uploaded config file to project directory. Check permissions.\n")
        removeModal()
        return()
      }
      # Record path and prepare to pass to processx
      uploaded_saved_path <- dest_path
      config_arg <- c("-config", dest_path)
      rv$log <- paste0(rv$log, "Uploaded config file accepted and saved to: ", dest_path, "\n")
    }
    # === END CHANGED ===
    
    # Run the Go executable (working directory is project_dir)
    # === CHANGED/ADDED ===
    # Pass config_arg as args (empty if no upload)
    run_res <- tryCatch({
      processx::run(command = exec_cmd, args = config_arg, wd = project_dir,
                    timeout = default_timeout, echo = FALSE, spinner = FALSE)
    }, error = function(e) {
      # processx may throw errors (timeout etc.); include error message in stderr field
      list(status = -1, stdout = ifelse(!is.null(e$stdout), e$stdout, ""), stderr = e$message)
    })
    # === END CHANGED ===
    
    # Combine log information
    combined_log <- paste0(
      "=== stdout ===\n",
      ifelse(is.null(run_res$stdout), "", run_res$stdout),
      "\n\n=== stderr ===\n",
      ifelse(is.null(run_res$stderr), "", run_res$stderr),
      "\n\n=== exit status ===\n",
      ifelse(is.null(run_res$status), "unknown", as.character(run_res$status)),
      "\n\n"
    )
    rv$log <- combined_log
    
    # Find the latest two gifs (after run)
    latest_paths <- find_latest_gifs(project_dir, n = 2)
    
    if (length(latest_paths) == 0) {
      rv$log <- paste0(rv$log, "No .gif files found in project directory or subdirectories.\n")
      rv$latest_gifs <- character(0)
      rv$gif_infos <- list()
      # Clean up uploaded config if exists
      if (!is.null(uploaded_saved_path) && file.exists(uploaded_saved_path)) {
        try(file.remove(uploaded_saved_path), silent = TRUE)
      }
      removeModal()
      return()
    }
    
    # Copy found gifs to app's www directory, keep extension and timestamp to avoid caching issues
    ts <- format(Sys.time(), "%Y%m%d%H%M%S")
    copied_names <- character(0)
    copy_report <- c()
    for (i in seq_along(latest_paths)) {
      src <- latest_paths[i]
      # basename (includes .gif); sanitize but preserve extension
      orig_basename <- basename(src)
      # ensure extension is .gif (lowercase)
      ext <- tools::file_ext(orig_basename)
      if (tolower(ext) != "gif") {
        # if original lacked .gif extension, force .gif
        safe_base <- paste0(gsub("[^A-Za-z0-9_.-]", "_", tools::file_path_sans_ext(orig_basename)), ".gif")
      } else {
        safe_base <- gsub("[^A-Za-z0-9_.-]", "_", orig_basename)
      }
      dest_name <- paste0(sprintf("gif_%d_%s_", i, ts), safe_base)
      dest_path <- file.path(app_www, dest_name)
      ok <- tryCatch({
        file.copy(src, dest_path, overwrite = TRUE)
      }, error = function(e) FALSE)
      copy_report <- c(copy_report, paste0("copy ", src, " -> ", dest_path, " : ", ifelse(ok, "OK", "FAIL")))
      if (ok) copied_names <- c(copied_names, dest_name)
    }
    
    # Append copy report to log for troubleshooting
    rv$log <- paste0(rv$log, "\nCopy report:\n", paste(copy_report, collapse = "\n"), "\n\n")
    # List gifs under www for confirmation
    www_list <- tryCatch({
      l <- list.files(app_www, pattern = "\\.gif$", ignore.case = TRUE)
      paste0("GIF list under www/:\n", paste(l, collapse = "\n"))
    }, error = function(e) paste0("Failed to list www directory: ", e$message))
    rv$log <- paste0(rv$log, www_list, "\n")
    
    # If copying failed (no files copied to www), fall back to original absolute paths (may not be accessible in browser)
    if (length(copied_names) == 0) {
      rv$log <- paste0(rv$log, "\nWarning: copying to www failed, will attempt to reference original paths (may not display in some browsers/deployments).\n")
      rv$latest_gifs <- latest_paths  # absolute paths
    } else {
      rv$latest_gifs <- copied_names   # names under www (accessible via /gifs/<name>)
    }
    
    # For each gif gather metadata where possible
    infos_list <- list()
    for (gname in rv$latest_gifs) {
      # gname might be a www-relative filename or an absolute path
      fullpath <- if (file.exists(file.path(app_www, gname))) {
        file.path(app_www, gname)
      } else if (file.exists(gname)) {
        # absolute path
        gname
      } else {
        # if copying failed but gname equals basename(latest_paths), try to match original latest_paths
        matched <- latest_paths[basename(latest_paths) == gname]
        if (length(matched) > 0) matched[1] else NA
      }
      if (!is.na(fullpath) && file.exists(fullpath)) {
        info <- tryCatch({
          img <- image_read(fullpath)
          iinfo <- image_info(img)
          list(
            display_name = basename(fullpath),
            path = fullpath,
            width = iinfo$width[1],
            height = iinfo$height[1],
            frames = nrow(iinfo),
            size_kb = round(file.info(fullpath)$size / 1024, 2),
            duration = if ("delay" %in% colnames(iinfo)) round(sum(iinfo$delay, na.rm = TRUE) / 100, 2) else NA
          )
        }, error = function(e) {
          list(display_name = basename(fullpath), path = fullpath, width = NA, height = NA, frames = NA, size_kb = NA, duration = NA)
        })
      } else {
        info <- list(display_name = as.character(gname), path = NA, width = NA, height = NA, frames = NA, size_kb = NA, duration = NA)
      }
      infos_list[[length(infos_list) + 1]] <- info
    }
    rv$gif_infos <- infos_list
    
    # === CHANGED/ADDED ===
    # Attempt to remove uploaded saved config after run to avoid accumulation
    if (!is.null(uploaded_saved_path) && file.exists(uploaded_saved_path)) {
      try({
        file.remove(uploaded_saved_path)
        rv$log <- paste0(rv$log, "\nDeleted temporary saved config file: ", uploaded_saved_path, "\n")
      }, silent = TRUE)
    }
    # === END CHANGED ===
    
    removeModal()
  })
  
  # Run log UI
  output$run_log <- renderText({ rv$log })
  
  # Dynamically render up to two gifs (or fewer)
  output$gif_ui <- renderUI({
    gifs <- rv$latest_gifs
    if (length(gifs) == 0) {
      div(style = "color: #777;", "No GIF generated yet, or none found after run. Click Run Simulation to start.")
    } else {
      ts <- as.integer(Sys.time())
      imgs <- lapply(seq_along(gifs), function(i) {
        g <- gifs[i]
        # Prefer serving as www file (via addResourcePath -> /gifs/<name>)
        if (file.exists(file.path(app_www, g))) {
          src <- paste0("gifs/", g, "?t=", ts)  # use mapping prefix gifs/
        } else if (file.exists(g)) {
          # absolute path (not recommended) — try file:// protocol
          src <- paste0("file://", normalizePath(g), "?t=", ts)
        } else {
          src <- NULL
        }
        if (!is.null(src)) {
          tags$div(style = "display:inline-block; margin-right:12px; vertical-align:top;",
                   tags$h5(paste0("GIF ", i)),
                   tags$img(src = src, style = "max-width:420px; height:auto; border:1px solid #ccc; padding:4px;"))
        } else {
          tags$div(style = "display:inline-block; margin-right:12px; vertical-align:top; color:#999;",
                   tags$h5(paste0("GIF ", i)), "Cannot display (file missing or inaccessible)")
        }
      })
      do.call(tagList, imgs)
    }
  })
  
  # GIF metadata table
  output$gif_meta <- renderTable({
    infos <- rv$gif_infos
    if (length(infos) == 0) return(NULL)
    df <- data.frame(
      File = sapply(infos, function(x) x$display_name),
      Width_px = sapply(infos, function(x) x$width),
      Height_px = sapply(infos, function(x) x$height),
      Frames = sapply(infos, function(x) x$frames),
      Size_KB = sapply(infos, function(x) x$size_kb),
      EstimatedDuration_s = sapply(infos, function(x) ifelse(is.na(x$duration), NA, x$duration)),
      stringsAsFactors = FALSE,
      check.names = FALSE
    )
    df
  }, striped = TRUE, spacing = "s")
  
  # Dynamically generate download buttons (one per gif)
  output$download_ui <- renderUI({
    gifs <- rv$latest_gifs
    if (length(gifs) == 0) return(NULL)
    buttons <- lapply(seq_along(gifs), function(i) {
      g <- gifs[i]
      btn_id <- paste0("download_gif_", i)
      output[[btn_id]] <- downloadHandler(
        filename = function() {
          if (file.exists(file.path(app_www, g))) return(basename(g))
          if (file.exists(g)) return(basename(g))
          paste0("gif_", i, ".gif")
        },
        content = function(file) {
          if (file.exists(file.path(app_www, g))) {
            file.copy(file.path(app_www, g), file, overwrite = TRUE)
          } else if (file.exists(g)) {
            file.copy(g, file, overwrite = TRUE)
          } else {
            stop("File does not exist, cannot download")
          }
        },
        contentType = "image/gif"
      )
      downloadButton(btn_id, label = paste0("Download GIF ", i), style = "margin-bottom:6px;")
    })
    do.call(tagList, buttons)
  })
}

shinyApp(ui, server)
