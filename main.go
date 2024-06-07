package main

import (
	"bytes"
	"context"
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
	"github.com/gorilla/handlers"
	"github.com/jung-kurt/gofpdf"
	"github.com/nfnt/resize"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Peacemaker struct {
	Name        string `json:"name"`
	Wallet      string `json:"wallet"`
	Citizenship string `json:"citizenship"`
	Language    string `json:"language"`
}

type RequestData struct {
	Peacemakers []Peacemaker `json:"peacemakers"`
}

type Template struct {
	Name      string `json:"name"`
	Language  string `json:"language"`
	CreatedBy string `json:"created_by"`
	Path      string `json:"path"`
}

type NewTemplate struct {
	Name      string `json:"name"`
	Language  string `json:"language"`
	CreatedBy string `json:"created_by"`
	Path      string `json:"path"`
}

type NewUser struct {
	FirstName    string   `json:"first_name"`
	LastName     string   `json:"last_name"`
	NickName     string   `json:"nick_name"`
	Email        string   `json:"email"`
	Wallet       string   `json:"wallet"`
	Languages    []string `json:"languages"`
	Citizenships []string `json:"citizenships"`
}

type User struct {
	FirstName    string   `json:"first_name"`
	LastName     string   `json:"last_name"`
	NickName     string   `json:"nick_name"`
	Email        string   `json:"email"`
	Wallet       string   `json:"wallet"`
	Languages    []string `json:"languages"`
	Citizenships []string `json:"citizenships"`
}

func main() {

	// err := godotenv.Load()
	// if err != nil {
	// 	log.Fatalf("Error loading .env file: %v", err)
	// }

	// Get the MongoDB URL from environment variables
	mongoDBURL := os.Getenv("MONGO_DB_URL")
	if mongoDBURL == "" {
		log.Fatal("MONGO_DB_URL environment variable not set.")
	}

	// Initialize the MongoDB client
	clientOptions := options.Client().ApplyURI(mongoDBURL)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Verify the connection
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB!")

	log.Println("Configuring CORS")

	corsMiddleware := handlers.CORS(
		handlers.AllowedOrigins([]string{
			"https://diplomacy.network",
			"http://localhost:3000",
		}),
		handlers.AllowedMethods([]string{
			http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions, // Specify allowed methods
		}),
		handlers.AllowedHeaders([]string{
			"Content-Type", "Access-Control-Allow-Origin",
		}),
	)

	wrapperPeace := func(w http.ResponseWriter, r *http.Request) {
		handler := corsMiddleware(http.HandlerFunc(handleCreateCertificates))
		handler.ServeHTTP(w, r)
	}
	wrapperRecognition := func(w http.ResponseWriter, r *http.Request) {
		handler := corsMiddleware(http.HandlerFunc(handleCreateCertificates))
		handler.ServeHTTP(w, r)
	}
	wrapperTemplate := func(w http.ResponseWriter, r *http.Request) {
		handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handleCreateTemplate(context.Background(), client, w, r)
		}))
		handler.ServeHTTP(w, r)
	}
	wrapperUser := func(w http.ResponseWriter, r *http.Request) {
		handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handleCreateUser(context.Background(), client, w, r)
		}))
		handler.ServeHTTP(w, r)
	}

	// Use the wrapper functions with http.HandleFunc
	http.HandleFunc("POST /peace", wrapperPeace)
	http.HandleFunc("POST /recognition", wrapperRecognition)
	http.HandleFunc("POST /template", wrapperTemplate)
	http.HandleFunc("POST /user", wrapperUser)

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

	endpoint := r.URL.Path
	var templateKind string

	switch endpoint {
	case "/peace":
		templateKind = "peace"
	case "/recognition":
		templateKind = "recognition"
	default:
		http.NotFound(w, r)
		return
	}
	log.Printf(" @handleCreateCertificates >>  template kind resolved as %s", templateKind)

	id := uuid.New()
	for _, peacemaker := range requestData.Peacemakers {
		log.Printf("Creating certificate for %s with template >> %s \n", peacemaker.Name, peacemaker.Language)
		if err := generateCertificate(templateKind, id.String(), requestData.Peacemakers, peacemaker); err != nil {
			log.Printf("Error generating certificate for %s: %v", peacemaker.Name, err)
			http.Error(w, fmt.Sprintf("Error generating certificate for %s", peacemaker.Name), http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprintf(w, "Certificates created successfully.\n")
}
func generateCertificate(kind string, uuidText string, peacemakers []Peacemaker, currentPeacemaker Peacemaker) error {

	var templateKind string
	if kind == "peace" {
		templateKind = "ProofOfPeacemaking"
	} else {
		templateKind = "ProofOfRecognition"
	}

	log.Printf(" @generateCertificate >>  template kind resolved as %s", templateKind)

	templatePath := fmt.Sprintf("./templates/%s_%s.jpg", templateKind, currentPeacemaker.Language)
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

	// var fileName  string
	fileName := fmt.Sprintf("%s_%s_%s.pdf", templateKind, currentPeacemaker.Name, currentPeacemaker.Wallet)
	return saveToPDF(img, fmt.Sprintf("./outcomes/%s", fileName), uuidText, float64(uuidHrefX), float64(uuidHrefY))
}

func handleCreateTemplate(ctx context.Context, client *mongo.Client, w http.ResponseWriter, r *http.Request) (*mongo.InsertOneResult, error) {
	var newTemplate NewTemplate
	if err := json.NewDecoder(r.Body).Decode(&newTemplate); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return nil, err
	}

	collection := client.Database("diplomacy-network").Collection("templates")
	result, err := collection.InsertOne(ctx, newTemplate)
	if err != nil {
		http.Error(w, "Failed to create new template", http.StatusInternalServerError)
		return nil, err
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
	return result, nil
}

func handleCreateUser(ctx context.Context, client *mongo.Client, w http.ResponseWriter, r *http.Request) (*mongo.InsertOneResult, error) {
	var newUser NewUser
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return nil, err
	}

	// Check if user with the same email or wallet already exists
	collection := client.Database("diplomacy-network").Collection("users")
	filter := bson.M{"email": newUser.Email, "wallet": newUser.Wallet}
	var existingUser User
	err := collection.FindOne(ctx, filter).Decode(&existingUser)
	if err == mongo.ErrNoDocuments {
		// No document found, proceed with insertion
		result, err := collection.InsertOne(ctx, newUser)
		if err != nil {
			http.Error(w, "Failed to create new user", http.StatusInternalServerError)
			return nil, err
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
		return result, nil
	} else if err != nil {
		// An error occurred other than no documents found
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil, err
	} else {
		// Document found, indicate conflict
		http.Error(w, "A user with this email or wallet already exists", http.StatusConflict)
		return nil, errors.New("conflict")
	}
}

// UTILS
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
