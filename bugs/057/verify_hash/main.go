package main
import (
    "fmt"
    "golang.org/x/crypto/bcrypt"
)
func main() {
    hash := "$2a$10$LMmanwGqjpWAS5LWrwEFY.8g2tl80y2I0.rU.gHdHCGNIAFOW/8uC"
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("memos"))
    fmt.Println("Error:", err)
}