package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB
var rdb *redis.Client
var ctx = context.Background()
var jwtKey = []byte("your_secret_key")

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"-"`
}
type UserReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func main() {
	initDb()
	defer db.Close()

	rdb = redis.NewClient(&redis.Options{
		Addr: "redis:6379", // Assumes Redis is running in a container named 'redis'
	})

	e := echo.New()

	e.Use(middleware.Recover())
	// Routes
	e.GET("", getUsers)
	e.GET("/:id", getUser)
	e.POST("", addUser)
	e.PUT("/:id", updateUser)
	e.DELETE("/:id", deleteUser)

	e.POST("/signup", signup)
	e.POST("/login", login)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3002"
	}
	// start server
	e.Logger.Fatal(e.Start(":" + port))
}

func signup(c echo.Context) error {
	var u UserReq
	if err := c.Bind(&u); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to hash password")
	}

	_, err = db.Exec("INSERT INTO users(name, email, password) VALUES(?, ?, ?)", u.Name, u.Email, string(hashedPassword))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create user")
	}

	return c.JSON(http.StatusCreated, "User created successfully")
}

func login(c echo.Context) error {
	var u UserReq
	if err := c.Bind(&u); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	var storedPassword string
	err := db.QueryRow("SELECT password FROM users WHERE email = ?", u.Email).Scan(&storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusUnauthorized, "Invalid credentials")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(u.Password)); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid credentials")
	}

	expirationTime := time.Now().Add(15 * time.Minute)
	claims := &Claims{
		Email: u.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create token")
	}

	err = rdb.Set(ctx, u.Email, tokenString, 15*time.Minute).Err()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to store token")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"token": tokenString,
	})
}

func initDb() {
	var err error
	db, err = sql.Open("sqlite3", "./users.db")
	if err != nil {
		log.Fatal(err)
	}
	createTableQuery := `CREATE TABLE IF NOT EXISTS users(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT NOT NULL,
		password TEXT NOT NULL
	);`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatal(err)
	}

	// Clear and seed initial data
	_, err = db.Exec("DELETE FROM users")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("INSERT INTO users(name, email,password) VALUES(?, ?,?)", "John Doe", "john@example.com", "123")
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO users(name, email,password) VALUES(?, ?,?)", "Jane Smith", "jane@example.com", "123")
	if err != nil {
		log.Fatal(err)
	}
}

func getUser(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user Id")
	}

	var u User
	err = db.QueryRow("SELECT id, name, email FROM users WHERE id = ?", id).Scan(&u.ID, &u.Name, &u.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, u)
}

func getUsers(c echo.Context) error {
	rows, err := db.Query("SELECT id, name, email FROM users")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		users = append(users, u)
	}

	return c.JSON(http.StatusOK, users)
}

func addUser(c echo.Context) error {
	var u User
	if err := c.Bind(&u); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	result, err := db.Exec("INSERT INTO users(name, email) VALUES(?, ?)", u.Name, u.Email)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	id, err := result.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	u.ID = int(id)
	return c.JSON(http.StatusCreated, u)
}

func updateUser(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user Id")
	}

	var u User
	if err := c.Bind(&u); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	result, err := db.Exec("UPDATE users SET name = ?, email = ? WHERE id = ?", u.Name, u.Email, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	u.ID = id
	return c.JSON(http.StatusOK, u)
}

func deleteUser(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user Id")
	}

	result, err := db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	return c.NoContent(http.StatusNoContent)
}
