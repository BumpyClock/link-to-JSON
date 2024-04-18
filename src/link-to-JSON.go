package main

import (
	"log"
	"net/http"
	URL "net/url"
	"os"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly"
	"github.com/joho/godotenv"
	"github.com/patrickmn/go-cache" // Caching package
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate" // Rate limiter
)

type ResponseItem struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Images      []WebImage `json:"images"`
	Sitename    string     `json:"sitename"`
	Favicon     string     `json:"favicon"`
	Duration    int        `json:"duration"`
	Domain      string     `json:"domain"`
	URL         string     `json:"url"`
}

type WebImage struct {
	URL    string `json:"url"`
	Alt    string `json:"alt,omitempty"`
	Type   string `json:"type,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

var (
	rateLimiter = rate.NewLimiter(1, 3) // Allows 1 request per second with a burst capacity of 3
	cch         = cache.New(30*time.Minute, 60*time.Minute)
	userAgent   string
)

func fetchMetadata(url string) (*ResponseItem, error) {
	// Check cache first
	if cached, found := cch.Get(url); found {
		return cached.(*ResponseItem), nil
	}

	c := colly.NewCollector()
	c.OnRequest(func(r *colly.Request) {
		logrus.Info("Visiting", r.URL)
		r.Headers.Set("User-Agent", userAgent)
	})
	result := &ResponseItem{URL: url, Images: []WebImage{}}
	result.Domain = getBaseDomain(url)
	webImage := WebImage{}

	c.OnHTML("title", func(e *colly.HTMLElement) {
		if result.Title == "" {
			result.Title = e.Text
		}
	})
	c.OnHTML(`meta[name="description"]`, func(e *colly.HTMLElement) {
		result.Description = e.Attr("content")
	})
	c.OnHTML(`link[rel="icon"], link[rel="shortcut icon"], link[rel="apple-touch-icon"], link[rel="apple-touch-icon-precomposed"]`, func(e *colly.HTMLElement) {
		if result.Favicon == "" {
			result.Favicon = result.Domain + e.Attr("href")
		}
	})
	c.OnHTML(`meta[property="og:site_name"]`, func(e *colly.HTMLElement) {
		result.Sitename = e.Attr("content")
	})
	c.OnHTML(`meta[property="og:image"]`, func(e *colly.HTMLElement) {
		webImage.URL = e.Attr("content")
	})
	c.OnHTML(`meta[property="og:image:alt"]`, func(e *colly.HTMLElement) {
		webImage.Alt = e.Attr("content")
	})
	c.OnHTML(`meta[property="og:image:type"]`, func(e *colly.HTMLElement) {
		webImage.Type = e.Attr("content")
	})
	c.OnHTML(`meta[property="og:image:width"]`, func(e *colly.HTMLElement) {
		width, err := strconv.Atoi(e.Attr("content"))
		if err == nil {
			webImage.Width = width
		}
	})
	c.OnHTML(`meta[property="og:image:height"]`, func(e *colly.HTMLElement) {
		height, err := strconv.Atoi(e.Attr("content"))
		if err == nil {
			webImage.Height = height
		}
	})
	c.OnScraped(func(r *colly.Response) {
		result.Images = append(result.Images, webImage)
		logrus.Info("Scraping finished", url)
	})

	// Handle visiting the URL
	err := c.Visit(url)
	if err != nil {
		logrus.Error("Failed to visit URL: ", err)
		return nil, err
	}

	if result.Sitename == "" {
		c2 := colly.NewCollector()
		logrus.Info("Visiting", result.Domain)
		c2.OnHTML(`meta[property="og:title"]`, func(e *colly.HTMLElement) {
			result.Sitename = e.Attr("content")

		})

		err = c2.Visit(result.Domain)
		if err != nil {
			logrus.Error("Failed to visit base domain: ", err)
			return nil, err
		}
	}

	// Cache the result
	cch.Set(url, result, cache.DefaultExpiration)

	return result, nil
}

func getBaseDomain(url string) string {
	parsedURL, err := URL.Parse(url)
	if err != nil {
		return ""
	}

	return parsedURL.Scheme + "://" + parsedURL.Host
}

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
		metadata, err := fetchMetadata(url)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch metadata"})
			return
		}

		duration := time.Since(startTime)
		metadata.Duration = int(duration.Milliseconds())

		c.JSON(http.StatusOK, metadata)
	})

	router.Run(":8080")
}
