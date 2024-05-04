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
	"github.com/google/uuid"
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

func main() {
	http.HandleFunc("POST /", handleCreateCertificates)
	log.Fatal(http.ListenAndServe(":3030", nil))
}

func handleCreateCertificates(w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	var requestData RequestData
	if err := json.Unmarshal(body, &requestData); err != nil {
		http.Error(w, "Error unmarshalling JSON", http.StatusBadRequest)
		return
	}

	// Generate a UUID
	id := uuid.New()
	for _, peacemaker := range requestData.Peacemakers {
		log.Printf("Creating certificate for %s with template >> %s \n", peacemaker.Name, peacemaker.Language)
		if err := generateCertificate(id.String(), requestData.Peacemakers, peacemaker); err != nil {
			log.Printf("Error generating certificate for %s: %v", peacemaker.Name, err)
			http.Error(w, fmt.Sprintf("Error generating certificate for %s", peacemaker.Name), http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprintf(w, "Certificates created successfully.\n")
}
func generateCertificate(uuidText string, peacemakers []Peacemaker, currentPeacemaker Peacemaker) error {
	templatePath := fmt.Sprintf("./templates/ProofOfPeacemaking_%s.jpg", currentPeacemaker.Language)
	img, err := loadTemplateImage(templatePath)
	if err != nil {
		return err
	}

	font, err := loadFont("./fonts/Platypi-VariableFont_wght.ttf")
	if err != nil {
		return err
	}

	// Draw names
	if len(peacemakers) > 1 {
		if err := drawText(img, peacemakers[0].Name, font, 480, 600, color.RGBA{0, 0, 0, 255}); err != nil {
			return err
		}
		if err := drawText(img, peacemakers[1].Name, font, 1120, 600, color.RGBA{0, 0, 0, 255}); err != nil {
			return err
		}
	}

	// Place flags in opposite corners
	if len(peacemakers) > 1 {
		if err := drawFlag(img, peacemakers[0].Citizenship, 10, 10); err != nil {
			return err
		}
		if err := drawFlag(img, peacemakers[1].Citizenship, img.Bounds().Dx()-90, 10); err != nil {
			return err
		}
	}

	// Draw UUID at the bottom left corner
	uuidHrefX := 50
	uuidHrefY := img.Bounds().Dy() - 50

	if err := drawText(img, uuidText, font, uuidHrefX, uuidHrefY, color.RGBA{0, 0, 0, 255}); err != nil {
		return err
	}

	return saveToPDF(img, fmt.Sprintf("./outcomes/ProofOfPeacemaking_%s.pdf", currentPeacemaker.Name), uuidText, float64(uuidHrefX), float64(uuidHrefY))
}

func loadTemplateImage(path string) (*image.RGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, err := jpeg.Decode(file)
	if err != nil {
		return nil, err
	}

	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)
	return rgba, nil
}

func loadFont(path string) (*truetype.Font, error) {
	fontBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return truetype.Parse(fontBytes)
}

func drawText(img *image.RGBA, text string, font *truetype.Font, x, y int, textColor color.Color) error {
	if img == nil || font == nil {
		return errors.New("nil image or font")
	}

	ctx := freetype.NewContext()
	ctx.SetDPI(72)
	ctx.SetFont(font)
	ctx.SetFontSize(40)
	ctx.SetClip(img.Bounds())
	ctx.SetDst(img)
	ctx.SetSrc(image.NewUniform(textColor))
	pt := freetype.Pt(x, y+int(ctx.PointToFixed(40)>>6)) // Y-offset is adjusted for baseline

	_, err := ctx.DrawString(text, pt)
	return err
}

func drawFlag(img *image.RGBA, countryCode string, x, y int) error {
	imagePath := fmt.Sprintf("./flags/%s.png", countryCode)
	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	flagImage, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	resizedImage := resize.Resize(80, 0, flagImage, resize.Lanczos3)
	draw.Draw(img, image.Rect(x, y, x+80, y+80), resizedImage, image.Point{0, 0}, draw.Over)

	return nil
}

// return saveToPDF(img, fmt.Sprintf("ProofOfPeacemaking_%s.pdf", currentPeacemaker.Name), id.String(), uuidHrefX, uuidHrefY)

func saveToPDF(img *image.RGBA, filename string, uuid string, uuidX float64, uuidY float64) error {
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, nil); err != nil {
		return err
	}

	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	opts := gofpdf.ImageOptions{
		ImageType: "JPEG",
		ReadDpi:   true,
	}

	pdf.RegisterImageOptionsReader(filename, opts, bytes.NewReader(buf.Bytes()))
	pdf.ImageOptions(filename, 0, 0, 297, 210, false, opts, 0, "")

	// Calculate the width of the UUID text in the PDF
	fontSize := 12.0 // match this with your actual font size used in drawText
	pdf.SetFont("Arial", "", fontSize)
	uuidWidth := pdf.GetStringWidth(uuid)

	// Add a hyperlink over the UUID text
	link := fmt.Sprintf("https://diplomacy.network/proofs-of-peacemaking/%s", uuid)
	pdf.LinkString(float64(uuidX), float64(uuidY), uuidWidth, fontSize, link)

	return pdf.OutputFileAndClose(filename)
}
