package main

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

var pidOfNode int

func main() {

	os.Args = append(os.Args, "serve")
	os.Args = append(os.Args, "--dir", "capitano_data")
	os.Args = append(os.Args, "--http", "0.0.0.0:80")

	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.SetPrefix("[Capitano] ")

	// Ignore the os.Interrupt signal (Ctrl+C)
	signal.Ignore(os.Interrupt)

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt, syscall.SIGTERM)

	// Does pgrep exist?
	_, err := exec.LookPath("pgrep")
	if err != nil {
		log.Println("Your website is ready to publish. See https://capitano.dev for more information.")
	} else {
		startWebsite()
	}

	app := pocketbase.New()

	// Add the migrate command
	migratecmd.MustRegister(app, app.RootCmd, &migratecmd.Options{
		Automigrate: true,
		Dir:         "capitano_migrations",
	})

	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {

		// Post to / to deploy a new version of the website.
		e.Router.AddRoute(echo.Route{
			Method:  http.MethodPost,
			Path:    "/",
			Handler: handleDeploy,
			Middlewares: []echo.MiddlewareFunc{
				apis.ActivityLogger(app),
				apis.RequireAdminAuth(),
			},
		})
		e.Router.AddRoute(echo.Route{
			Method:  http.MethodGet,
			Path:    "/*",
			Handler: handleProxy,
			Middlewares: []echo.MiddlewareFunc{
				apis.ActivityLogger(app),
			},
		})
		return nil
	})

	// Start the framework
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}

	<-terminate // Wait for the termination signal

	log.Println("Received reconfiguration signal. Shutting down the app server...")
	os.Exit(0)
}

func handleProxy(c echo.Context) error {
	proxyURL, err := url.Parse("http://localhost:3000")
	if err != nil {
		return err
	}

	proxy := httputil.NewSingleHostReverseProxy(proxyURL)
	req := c.Request()
	req.Header.Set("X-Forwarded-For", c.RealIP())

	// Perform a HEAD request to check if the target URL is reachable
	resp, err := http.Head(proxyURL.String())
	if err != nil || resp.StatusCode != http.StatusOK {
		c.Response().Write([]byte("The website is ready to be published! Visit https://capitano.dev for more information."))
		return nil
	}

	proxy.ServeHTTP(c.Response(), req)
	return nil
}

func startWebsite() {

	// Does /app/dist/index.js exist?
	if _, err := os.Stat("/app/dist/index.js"); os.IsNotExist(err) {
		return
	}

	cmd := exec.Command("node", "/app/dist/index.js")

	cmd.Dir = ""

	// Redirect the command's output to the current process's output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the child process and set the pidOfNode variable
	err := cmd.Start()

	pidOfNode = cmd.Process.Pid

	if err != nil {
		fmt.Println("Error starting child process:", err)
		return
	}

	fmt.Println("Node server process started with PID:", pidOfNode)
}

func handleDeploy(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return err
	}

	// Check if the file is a tar file
	if !strings.HasSuffix(file.Filename, ".tar") {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid file format. Only .tar files are allowed.")
	}

	// Create a new file in the current directory
	dst, err := os.Create(file.Filename)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// Copy the contents of the uploaded file to the newly created file
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	// Is pidOfNode set?
	if pidOfNode != 0 {
		// Verify if the process is still running
		_, err := os.FindProcess(pidOfNode)
		if err != nil {
			fmt.Println("No website process found. Starting deployment...")
			pidOfNode = 0
		} else {
			fmt.Println("Safely shutting down the website...")

			// Find the process by ID
			process, err := os.FindProcess(pidOfNode)
			if err != nil {
				fmt.Printf("Unable to find a process with the specified ID %d: %s\n", pidOfNode, err)
			}

			err = process.Signal(syscall.SIGTERM)
			if err != nil {
				fmt.Printf("Unable to terminate the process with the specified ID. %d: %s\n", pidOfNode, err)
			}

			fmt.Printf("The process with ID %d has been terminated successfully.\n", pidOfNode)
		}
	}

	destinationPath := "/app/dist"

	// Remove everything in /app/dist
	err = os.RemoveAll(destinationPath)
	if err != nil {
		log.Println("Failed to remove /app/dist:", err)
	}

	// Remove all .json files in /app
	err = filepath.Walk("/app", func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".json") {
			errRemove := os.Remove(path)
			if errRemove != nil {
				log.Println("Failed to remove .json file:", errRemove)
			}
		}
		return nil // This just returns nil to continue walking the directory
	})

	if err != nil {
		log.Println("Error walking the directory:", err)
	}

	err = unpackTarArchive(file.Filename, "/app")
	if err != nil {
		log.Println(err)
	}

	// Run the npm ci --omit dev
	cmd := exec.Command("npm", "ci", "--omit", "dev")
	err = cmd.Run()
	if err != nil {
		log.Println("Error running npm ci:", err)
	}

	startWebsite()

	return c.String(http.StatusOK, "App successfully published!")

}

func unpackTarArchive(archive, destinationPath string) error {
	file, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("failed to open archive: %v", err)
	}
	defer file.Close()

	tarReader := tar.NewReader(file)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // Reached the end of the archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %v", err)
		}

		targetPath := filepath.Join(destinationPath, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(targetPath, 0755)
			if err != nil {
				return fmt.Errorf("failed to create directory: %v", err)
			}
		case tar.TypeReg:
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to create file: %v", err)
			}
			defer file.Close()

			_, err = io.Copy(file, tarReader)
			if err != nil {
				return fmt.Errorf("failed to extract file: %v", err)
			}
		default:
			return fmt.Errorf("unknown tar entry type: %v", header.Typeflag)
		}
	}

	return nil
}
