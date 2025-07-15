package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// ServiceConfig holds configuration for each microservice
type ServiceConfig struct {
	Name     string
	BasePath string
	Targets  []string
}
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

var (
	services = []ServiceConfig{
		{
			Name:     "product-service", // Logical name for the group
			BasePath: "/api/products",
			Targets:  []string{"http://product-service-1:3001", "http://product-service-2:3003"},
		},
		{
			Name:     "user-service", // Logical name for the group
			BasePath: "/api/users",
			Targets:  []string{"http://user-service-1:3002", "http://user-service-2:3004"},
		},
		{
			Name:     "checkout-service",                       // Logical name for the checkout service
			BasePath: "/api/checkout",                          // Gateway path for checkout-related requests
			Targets:  []string{"http://checkout-service:3005"}, // Docker Compose service name and port
		},
	}

	// JWT secret key for signing and validating tokens
	jwtKey = []byte("your-secret-key")

	// Simple round-robin load balancer
	currentIndex = make(map[string]int)
)

func main() {

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.Use(middleware.Recover())

	// --- Custom Authentication Middleware ---
	// This middleware protects the /api/users routes by requiring a specific API key.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if strings.HasPrefix(c.Path(), "/api/checkout") { // Protect the checkout route
				authHeader := c.Request().Header.Get("Authorization")
				if authHeader == "" {
					return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorization header")
				}

				tokenString := strings.TrimPrefix(authHeader, "Bearer ")
				claims := &Claims{} // You need the Claims struct here as well

				token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
					return jwtKey, nil // And the jwtKey
				})

				if err != nil || !token.Valid {
					return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
				}

				// Optional: Check if the token is in Redis for an extra layer of security
			}
			return next(c)
		}
	})

	// Route all requests through the gateway
	e.Any("/api/users", gatewayHandler)
	e.Any("/api/products", gatewayHandler)
	e.Any("/api/checkout", gatewayHandler)

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}

func gatewayHandler(c echo.Context) error {
	path := c.Request().URL.Path

	// Find which service should handle this request
	var service *ServiceConfig
	for _, s := range services {
		if strings.HasPrefix(path, s.BasePath) {
			service = &s
			break
		}
	}

	if service == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Service not found")
	}

	// Simple round-robin load balancing
	idx := currentIndex[service.Name]
	target := service.Targets[idx]
	currentIndex[service.Name] = (idx + 1) % len(service.Targets)
	fmt.Println(target)

	// Prepare the request to the target service
	//req, err := http.NewRequest(c.Request().Method, target+path, c.Request().Body)
	trimmedPath := strings.TrimPrefix(path, service.BasePath)
	if !strings.HasPrefix(trimmedPath, "/") {
		trimmedPath = "/" + trimmedPath
	}
	req, err := http.NewRequest(c.Request().Method, target+trimmedPath, c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Copy headers
	for name, values := range c.Request().Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Make the request with retry logic for fault tolerance
	client := &http.Client{Timeout: 5 * time.Second}
	var resp *http.Response
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		resp, err = client.Do(req)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond) // Exponential backoff
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			c.Response().Header().Add(name, value)
		}
	}

	// Set status code
	c.Response().Writer.WriteHeader(resp.StatusCode)

	// Copy response body
	_, err = io.Copy(c.Response().Writer, resp.Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return nil
}
