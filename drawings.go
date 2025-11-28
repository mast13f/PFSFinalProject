package main

import (
	"canvas"
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"os"
)

func AnimateEnvironment(timePoints []*Environment, canvasWidth, frequency int, scalingFactor float64) []image.Image {
	images := make([]image.Image, 0)

	if len(timePoints) == 0 {
		panic("Error: no Environment objects present in AnimateEnvironment.")
	}

	for i := range timePoints {
		if i%frequency == 0 {
			fmt.Println("frame", i)
			images = append(images, timePoints[i].DrawToCanvas(canvasWidth, scalingFactor))
		}
	}

	return images
}

// DrawToCanvas는 Environment 하나(한 시점)를 캔버스에 그림.
// canvasWidth x canvasWidth 픽셀의 정사각형, 검은 배경.
// scalingFactor는 한 사람 점 반지름(px)을 조절하는 용도로 사용.
func (env *Environment) DrawToCanvas(canvasWidth int, scalingFactor float64) image.Image {
	if env == nil {
		panic("Can't Draw a nil Environment.")
	}

	// 새 캔버스 생성
	c := canvas.CreateNewCanvas(canvasWidth, canvasWidth)

	// 배경: 검은색
	c.SetFillColor(canvas.MakeColor(0, 0, 0))
	c.ClearRect(0, 0, canvasWidth, canvasWidth)
	c.Fill()

	if env.areaSize <= 0 {
		// areaSize가 비정상이라면 1로 가정
		env.areaSize = 1.0
	}

	// 각 개체(individual)를 원으로 그림
	for _, ind := range env.population {
		if ind == nil {
			continue
		}

		// 상태에 따른 색 지정
		r, g, b := colorForHealthStatus(ind)

		c.SetFillColor(canvas.MakeColor(r, g, b))

		// 위치: [0, areaSize] → [0, canvasWidth] 로 선형 스케일링
		cx := (ind.position.x / env.areaSize) * float64(canvasWidth)
		cy := (ind.position.y / env.areaSize) * float64(canvasWidth)

		// 반지름: scalingFactor를 픽셀 단위로 사용 (너무 작으면 잘 안보이니 기본값 2~4 추천)
		radius := scalingFactor
		if radius <= 0 {
			radius = 2.0
		}

		c.Circle(cx, cy, radius)
		c.Fill()
	}

	// image.Image로 반환
	return c.GetImage()
}

// healthStatus에 따라 RGB 색을 정하는 helper
func colorForHealthStatus(ind *Individual) (uint8, uint8, uint8) {
	if ind == nil {
		return 255, 255, 255 // fallback: white
	}

	switch ind.healthStatus {
	case Healthy:
		return 0, 255, 0 // 초록
	case Susceptible:
		return 255, 255, 0 // 노랑
	case Infected:
		return 255, 0, 0 // 빨강
	case Recovered:
		return 0, 128, 255 // 파랑 계열
	case Dead:
		return 160, 160, 160 // 회색
	default:
		return 255, 255, 255 // unknown: white
	}
}

// SaveEnvironmentGIF는 프레임들(images)을 GIF로 저장한다.
// delay는 각 프레임 사이의 딜레이 (1/100초 단위, ex. 5 = 0.05초)
func SaveEnvironmentGIF(filename string, frames []image.Image, delay int) error {
	if len(frames) == 0 {
		return fmt.Errorf("no frames to save")
	}

	outGIF := &gif.GIF{}

	for _, img := range frames {
		bounds := img.Bounds()
		// GIF는 팔레트 이미지 필요 → Plan9 팔레트 사용
		paletted := image.NewPaletted(bounds, palette.Plan9)
		draw.Draw(paletted, bounds, img, bounds.Min, draw.Src)

		outGIF.Image = append(outGIF.Image, paletted)
		outGIF.Delay = append(outGIF.Delay, delay)
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := gif.EncodeAll(f, outGIF); err != nil {
		return err
	}
	return nil
}
