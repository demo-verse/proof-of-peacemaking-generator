package main

import (
	"bytes"
	"encoding/json"
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

type NameOffset struct {
	X uint16
	Y uint16
}

type HandlerFunc func(http.ResponseWriter, *http.Request)

const (
	templatePathPrefix = "./templates/ProofOfPeacemaking_%s.jpg"
	fontPath           = "./fonts/Platypi-VariableFont_wght.ttf"
	flagsPathPrefix    = "./flags/%s.png"
)

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

		if err := generateCertificateForPeacemaker(peacemaker, requestData.Peacemakers); err != nil {
			log.Printf("Error generating certificate for %s: %v", peacemaker.Name, err)
			http.Error(w, "Error generating certificate", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Single certificate created successfully.\n")
}

func generateCertificateForPeacemaker(peacemaker Peacemaker, peacemakers []Peacemaker) error {
	templatePath := fmt.Sprintf(templatePathPrefix, peacemaker.Language)
	if err := generateCertificateForTemplate(templatePath, peacemaker, peacemakers); err != nil {
		return err
	}

	return nil
}

func generateCertificateForTemplate(templatePath string, peacemaker Peacemaker, peacemakers []Peacemaker) error {
	// Open the template image
	img, err := openTemplateImage(templatePath)
	if err != nil {
		return err
	}

	// Convert image to RGBA
	rgba := convertImageToRGBA(img)
	// Load the font
	font, err := loadFont(fontPath)
	if err != nil {
		return err
	}

	for _, peacemaker := range peacemakers {
		if peacemakers[0] == peacemaker {
			err = drawTextOnImage(rgba, peacemaker.Name, font, 480, 600, color.RGBA{0, 0, 0, 255})
			if err != nil {
				return err
			}

			err = drawImagesOnCorners(rgba, peacemaker.Citizenship, 0)
			if err != nil {
				return err
			}
		} else {
			err = drawTextOnImage(rgba, peacemaker.Name, font, 1120, 600, color.RGBA{0, 0, 0, 255})
			if err != nil {
				return err
			}

			err = drawImagesOnCorners(rgba, peacemaker.Citizenship, 1)
			if err != nil {
				return err
			}
		}
		// Draw text onto the image
	}

	// Convert image to buffer
	buf, err := convertImageToBuffer(rgba)
	if err != nil {
		return err
	}

	// Create a new PDF document and add the image
	pdf, err := createPDFWithImage(buf, &peacemaker)
	if err != nil {
		return err
	}

	// Save the PDF to a file
	err = savePDFToFile(pdf, fmt.Sprintf("ProofOfPeacemaking_%s.pdf", peacemaker.Name))
	if err != nil {
		return err
	}

	return nil
}

// helper funcs

func openTemplateImage(templatePath string) (*image.RGBA, error) {
	file, err := os.Open(templatePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)

	return rgba, nil
}

func convertImageToRGBA(img image.Image) *image.RGBA {
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)
	return rgba
}

func loadFont(fontPath string) (*truetype.Font, error) {
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, err
	}

	font, err := truetype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}

	return font, nil
}

func drawTextOnImage(img *image.RGBA, text string, font *truetype.Font, x, y int, textColor color.RGBA) error {
	d := freetype.NewContext()
	d.SetDPI(72)
	d.SetFont(font)
	d.SetFontSize(40)
	d.SetClip(img.Bounds())
	d.SetDst(img)
	d.SetSrc(image.NewUniform(textColor))

	pt := freetype.Pt(x, y+int(d.PointToFixed(0)>>10)) // calculate the baseline from the top
	_, err := d.DrawString(text, pt)
	return err
}

func drawImagesOnCorners(img *image.RGBA, citizenship string, drawOrder int) error {
	// Assuming flags are stored in a directory and named after the citizenship
	imagePath := fmt.Sprintf(flagsPathPrefix, citizenship)
	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	imageData, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	originalWidth := imageData.Bounds().Dx()
	originalHeight := imageData.Bounds().Dy()
	newWidth := 80
	newHeight := (originalHeight * newWidth) / originalWidth

	resizedImage := resize.Resize(uint(newWidth), uint(newHeight), imageData, resize.Lanczos3)

	dstRect := image.Rect(0, 0, newWidth, newHeight) // Adjust these values based on your layout

	cornerWidth := 100
	if drawOrder == 0 {
		draw.Draw(img, dstRect, resizedImage, image.Point{0, 0}, draw.Over)
	} else {
		draw.Draw(img, dstRect, resizedImage, image.Point{img.Bounds().Dx() - cornerWidth, 0}, draw.Over)

	}

	return nil
}

func convertImageToBuffer(img *image.RGBA) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, nil)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func createPDFWithImage(buf *bytes.Buffer, peacemaker *Peacemaker) (*gofpdf.Fpdf, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()

	opts := gofpdf.ImageOptions{
		ImageType: "JPEG",
		ReadDpi:   true,
	}

	pdf.RegisterImageOptionsReader(fmt.Sprintf("ProofOfPeacemaking_%s", peacemaker.Name)+".jpg", opts, bytes.NewReader(buf.Bytes()))
	pdf.ImageOptions(fmt.Sprintf("ProofOfPeacemaking_%s", peacemaker.Name)+".jpg", 0, 0, 297, 210, false, opts, 0, "")

	return pdf, nil
}

func savePDFToFile(pdf *gofpdf.Fpdf, fileName string) error {
	err := pdf.OutputFileAndClose(fileName)
	return err
}

var nameOffsets []NameOffset

func init() {
	nameOffsets = append(nameOffsets, NameOffset{X: uint16(480), Y: uint16(600)})
	nameOffsets = append(nameOffsets, NameOffset{X: uint16(1120), Y: uint16(600)})
}

func main() {
	RegisterRoutes()
	log.Fatal(http.ListenAndServe(":3030", nil))
}
