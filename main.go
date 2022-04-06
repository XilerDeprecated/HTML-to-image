package main

import (
	"bytes"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/timeout"
	"log"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

type Cookie struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Crop struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type Config struct {
	Format            string   `json:"format"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	DisableSmartWidth bool     `json:"disableSmartWidth"`
	Encoding          string   `json:"encoding"`
	Crop              Crop     `json:"crop"`
	Quality           int      `json:"quality"`
	Transparent       bool     `json:"transparent"`
	Cookies           []Cookie `json:"cookies"`
}

type GenerateImage struct {
	Html   string `json:"html"`
	Config Config `json:"config"`
}

var ValidFormats = map[string]struct{}{
	"png":  {},
	"jpg":  {},
	"jpeg": {},
	"svg":  {},
	"bmp":  {},
}

func isValidFormat(format string) bool {
	_, ok := ValidFormats[format]
	return ok
}

func generateArguments(config Config) ([]string, error) {
	var arguments []string

	if !isValidFormat(config.Format) {
		return nil, fmt.Errorf("invalid format: %s", config.Format)
	}

	if config.Format != "" {
		arguments = append(arguments, "-f", config.Format)
	}

	if config.Width != 0 {
		arguments = append(arguments, "--width", fmt.Sprintf("%d", config.Width))
	}

	if config.Height != 0 {
		arguments = append(arguments, "--height", fmt.Sprintf("%d", config.Height))
	}

	if config.DisableSmartWidth {
		arguments = append(arguments, "--disable-smart-width")
	}

	if config.Encoding != "" {
		arguments = append(arguments, "--encoding", config.Encoding)
	}

	if config.Quality != 0 {
		arguments = append(arguments, "--quality", fmt.Sprintf("%d", config.Quality))
	}

	if config.Transparent {
		arguments = append(arguments, "--transparent")
	}

	if config.Crop.X != 0 {
		arguments = append(arguments, "--crop-x", fmt.Sprintf("%d", config.Crop.X))
	}

	if config.Crop.Y != 0 {
		arguments = append(arguments, "--crop-y", fmt.Sprintf("%d", config.Crop.Y))
	}

	if config.Crop.H != 0 {
		arguments = append(arguments, "--crop-h", fmt.Sprintf("%d", config.Crop.H))
	}

	if config.Crop.W != 0 {
		arguments = append(arguments, "--crop-w", fmt.Sprintf("%d", config.Crop.W))
	}

	if len(config.Cookies) > 0 {
		for _, cookie := range config.Cookies {
			arguments = append(arguments, "--cookie", fmt.Sprintf("%s=%s", cookie.Key, url.QueryEscape(cookie.Value)))
		}
	}

	return arguments, nil
}

func handleGenerateImageRequest(c *fiber.Ctx) error {
	req := new(GenerateImage)

	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	args, err := generateArguments(req.Config)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	cmd := exec.Command("wkhtmltoimage", append(args, "-", "-")...)
	cmd.Stdin = strings.NewReader(req.Html)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		log.Println(err)
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Set("Content-Type", "image/"+req.Config.Format)
	return c.Send(out.Bytes())
}

func main() {
	app := fiber.New()
	app.Use(compress.New())
	app.Use(cache.New())
	app.Use(cors.New())
	app.Use(etag.New())
	app.Use(logger.New())

	app.Use(limiter.New(limiter.Config{
		Next: func(c *fiber.Ctx) bool {
			return "9c2221552d0d5cd960947f070850a4c7f72f0717237d05a0477f38bb7a98b5cb36e7d8703114c1e429ac9541f43ffaebb345" == c.GetReqHeaders()["Rate-Bypass"]
		},
		Max:               100,
		Expiration:        time.Hour * 24,
		LimiterMiddleware: limiter.SlidingWindow{},
	}))

	v1 := app.Group("/v1")
	v1.Post("/html-to-image", timeout.New(handleGenerateImageRequest, 5*time.Second))

	log.Fatal(app.Listen(":8080"))
}
