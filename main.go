package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var (
	uploadPath    = getEnv("UPLOAD_PATH", "./uploads")
	maxUploadSize = getEnvAsInt("MAX_UPLOAD_SIZE", 10*1024*1024) // 10 MB
	deletionDelay = getEnvAsInt("DELETION_DELAY", 5)             // 5 minutes
)

func main() {
	// Ensure upload directory exists.
	ensureUploadDir(uploadPath)

	http.HandleFunc("/share", uploadFileHandler())
	http.HandleFunc("/share/", downloadFileHandler())

	// Print the curl command to upload files in the log.
	fmt.Println("Server started at :9090")
	fmt.Printf("Use this command to upload files: 'curl -F \"file=@<file_path>\" http://localhost:9090/share'\n")

	http.ListenAndServe(":9090", nil)
}

func uploadFileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Restrict the size of the incoming file.
		r.Body = http.MaxBytesReader(w, r.Body, int64(maxUploadSize))
		if err := r.ParseMultipartForm(int64(maxUploadSize)); err != nil {
			http.Error(w, "File too big", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Invalid file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		filePath := filepath.Join(uploadPath, filepath.Base(header.Filename))
		out, err := os.Create(filePath)
		if err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		_, err = io.Copy(out, file)
		if err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}

		domainName := r.Host // Extract domain name from the request header.

		// Schedule file deletion.
		go deleteAfter(filePath, time.Duration(deletionDelay)*time.Minute)

		fmt.Fprintf(w, "File uploaded successfully: %s\n", header.Filename)
		fmt.Fprintf(w, "Use this command to download the file: 'curl %s/share/%s'\n", domainName, header.Filename)
	}
}

func downloadFileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fileName := r.URL.Path[len("/share/"):]

		filePath := filepath.Join(uploadPath, fileName)
		http.ServeFile(w, r, filePath)

		// Delete the file after successful download.
		go deleteFile(filePath)
	}
}

func deleteAfter(filePath string, delay time.Duration) {
	time.Sleep(delay)
	deleteFile(filePath)
}

func deleteFile(filePath string) {
	os.Remove(filePath) // Simplified error handling.
}

func ensureUploadDir(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(path, os.ModePerm)
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(name string, defaultValue int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}
