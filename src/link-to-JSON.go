package main

import (
	"log"
	"net/http"
	URL "net/url"
	"os"
	"time"

	link2json "github.com/BumpyClock/go-link2json"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv" // Caching package
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate" // Rate limiter
)

var (
	rateLimiter = rate.NewLimiter(1, 3) // Allows 1 request per second with a burst capacity of 3

)

func main() {
	router := gin.Default()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	userAgent := os.Getenv("LINK2JSON_USER_AGENT")
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"
		logrus.Warn("User agent not set, using default")
	} else {
		logrus.Info("User agent set to: ", userAgent)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	// Setup CORS
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	router.Use(cors.New(config))

	router.GET("/extract", func(c *gin.Context) {
		// Rate limit check
		if !rateLimiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			return
		}

		startTime := time.Now()
		url := c.Query("url")
		if url == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "URL parameter is required"})
			return
		}

		// Validate the URL
		_, err := URL.ParseRequestURI(url)
		if err != nil {
			logrus.Error("Invalid URL: ", url)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL"})
			return
		}

		metadata, err := link2json.GetMetadata(url)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch metadata"})
			return
		}

		duration := time.Since(startTime)
		metadata.Duration = int(duration.Milliseconds())

		c.JSON(http.StatusOK, metadata)
	})

	router.Run(":" + port)
	logrus.Info("Server started on port: ", port)

}
