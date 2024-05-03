package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/jung-kurt/gofpdf"
	"github.com/nfnt/resize"
)

type Peacemaker struct {
	Name        string `json:"name"`
	Citizenship string `json:"citizenship"`
	Language    string `json:"language"`
}

type RequestData struct {
	Peacemakers []Peacemaker `json:"peacemakers"`
}

type HandlerFunc func(http.ResponseWriter, *http.Request)

func RegisterRoutes() {
	http.HandleFunc("POST /", handleCreateSingleCertificate)
}

func handleCreateSingleCertificate(w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	var requestData RequestData
	err = json.Unmarshal(body, &requestData)
	if err != nil {
		http.Error(w, "Error unmarshalling JSON", http.StatusBadRequest)
		return
	}

	for _, peacemaker := range requestData.Peacemakers {
		fmt.Printf("Creating certificate for %s with template >> %s \n", peacemaker.Name, peacemaker.Language)
		fail := generateCertificate(requestData.Peacemakers, peacemaker.Name, peacemaker.Language)

		if fail != nil {
			log.Printf("Error generating certificate: %v", fail)
			http.Error(w, "Error generating certificate", http.StatusInternalServerError)
			return
		}
	}

	// fail := generateCertificate(requestData.Peacemakers, fmt.Sprintf("%s.pdf", outputFileName))

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Single certificate created successfully.\n")
}

func drawText(img *image.RGBA, text string, font *truetype.Font, x, y int, textColor color.Color) error {
	if img == nil {
		return errors.New("image is nil")
	}
	if font == nil {
		return errors.New("font is nil")
	}

	d := freetype.NewContext()
	d.SetDPI(72)
	d.SetFont(font)
	d.SetFontSize(40)
	d.SetClip(img.Bounds())
	d.SetDst(img)
	d.SetSrc(image.NewUniform(textColor))

	pt := freetype.Pt(x, y+int(d.PointToFixed(0)>>10)) // calculate the baseline from the top
	_, err := d.DrawString(text, pt)
	if err != nil {
		return err
	}
	return nil
}

func drawImage(img *image.RGBA, imagePath string, x, y int) error {
	if img == nil {
		return errors.New("image is nil")
	}

	// Open the provided image file
	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Decode the provided image
	imageData, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	originalWidth := imageData.Bounds().Dx()
	originalHeight := imageData.Bounds().Dy()
	newWidth := 80
	newHeight := (originalHeight * newWidth) / originalWidth

	// Resize the image to the new dimensions
	resizedImage := resize.Resize(uint(newWidth), uint(newHeight), imageData, resize.Lanczos3)

	// Calculate the destination rectangle for the resized image
	dstRect := image.Rect(x, y, x+newWidth, y+newHeight)

	// Draw the resized image onto the target image
	draw.Draw(img, dstRect, resizedImage, image.Point{0, 0}, draw.Over)

	return nil
}

func generateCertificate(peacemakers []Peacemaker, name string, language string) error {
	templatePath := fmt.Sprintf("./templates/ProofOfPeacemaking_%s.jpg", language)
	log.Printf("templatePath @ generateCertificate: %s", templatePath)
	file, err := os.Open(templatePath)
	if err != nil {
		return err
	}
	defer file.Close()

	img, err := jpeg.Decode(file)
	if err != nil {
		return err
	}

	// Convert image to RGBA
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)

	fontBytes, err := os.ReadFile("./fonts/Platypi-VariableFont_wght.ttf")
	if err != nil {
		return err
	}

	font, err := truetype.Parse(fontBytes)
	if err != nil {
		return err
	}

	textColor := color.RGBA{0, 0, 0, 255}

	// Draw text onto the image
	err = drawText(rgba, peacemakers[0].Name, font, 480, 600, textColor)
	if err != nil {
		return err
	}

	err = drawText(rgba, peacemakers[1].Name, font, 1120, 600, textColor)
	if err != nil {
		return err
	}

	// Draw images onto the corners
	cornerWidth := 100
	// cornerHeight := 100

	// Draw the first image at the top left corner
	err = drawImage(rgba, fmt.Sprintf("./flags/%s.png", peacemakers[0].Citizenship), 0, 0)
	if err != nil {
		return err
	}

	// Draw the second image at the top right corner
	err = drawImage(rgba, fmt.Sprintf("./flags/%s.png", peacemakers[1].Citizenship), rgba.Bounds().Dx()-cornerWidth, 0)

	if err != nil {
		return err
	}

	// Create a new PDF document
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	opts := gofpdf.ImageOptions{
		ImageType: "JPEG",
		ReadDpi:   true,
	}

	// Convert image to buffer
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, rgba, nil)
	if err != nil {
		return err
	}

	// Add image to PDF
	width, height := 297, 210 // Dimensions of A4 in landscape mode (297mm x 210mm)

	pdf.RegisterImageOptionsReader(fmt.Sprintf("ProofOfPeacemaking_%s", name)+".jpg", opts, bytes.NewReader(buf.Bytes()))
	pdf.ImageOptions(fmt.Sprintf("ProofOfPeacemaking_%s", name)+".jpg", 0, 0, float64(width), float64(height), false, opts, 0, "")

	return pdf.OutputFileAndClose(fmt.Sprintf("ProofOfPeacemaking_%s.pdf", name))
}

func main() {
	RegisterRoutes()
	log.Fatal(http.ListenAndServe(":3030", nil))
}
