package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type OllamaProcess struct {
	Models []Model `json:"models"`
}

type Model struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

var RootCommand = &cobra.Command{
	Use:              "ollamaUnloader",
	Short:            "unloads all running models from memory",
	Long:             "unloads all running models from memory",
	PersistentPreRun: rootPersistentPreRun,
	Run:              rootRun,
}

var ViperInstance = viper.New()

var ollamaHost string = "localhost:11434"

func init() {
	ViperInstance.AutomaticEnv()

	RootCommand.PersistentFlags().String("ollama-host", "localhost:11434", "Sets host address for ollama server")
	RootCommand.PersistentFlags().Bool("verbose", false, "Enables logs")

	// Bind flags to environment variables
	RootCommand.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		err := ViperInstance.BindPFlag(strings.ReplaceAll(flag.Name, "-", "_"), flag)
		if err != nil {
			log.Fatal(err)
		}
	})
}

func rootPersistentPreRun(_ *cobra.Command, _ []string) {
	ollamaHost = ViperInstance.GetString("ollama_host")
	verbose := ViperInstance.GetBool("verbose")
	if !verbose {
		log.SetOutput(io.Discard)
	}
}

func rootRun(_ *cobra.Command, _ []string) {
	ollamaProcess, err := getOllamaProcesses()
	if err != nil {
		panic(err)
	}

	allModels, err := extractModelsFromProcesses(ollamaProcess)
	if err != nil {
		panic(err)
	}

	err = unloadOllamaProcesses(allModels)
	if err != nil {
		panic(err)
	}
}

func extractModelsFromProcesses(ps OllamaProcess) ([]string, error) {
	log.Println("Extracting model names")

	allModels := []string{}
	for _, model := range ps.Models {
		allModels = append(allModels, model.Model)
	}

	if len(allModels) == 0 {
		return []string{}, errors.New("No models found")
	}

	return allModels, nil
}

func getOllamaProcesses() (OllamaProcess, error) {
	log.Println("Gathering all loaded models in memory")

	url := fmt.Sprintf("http://%s/api/ps", ollamaHost)
	resp, err := http.Get(url)
	if err != nil {
		return OllamaProcess{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return OllamaProcess{}, err
	}

	var ollamaProcess OllamaProcess
	err = json.Unmarshal(body, &ollamaProcess)
	if err != nil {
		return OllamaProcess{}, err
	}

	return ollamaProcess, nil
}

func unloadOllamaProcesses(models []string) error {
	url := fmt.Sprintf("http://%s/api/generate", ollamaHost)

	for _, model := range models {
		log.Println("Unloading model:", model)

		data, err := unloadRequestBodyGenerator(model)
		if err != nil {
			return err
		}

		r, err := http.Post(url, "application/json", data)
		if err != nil {
			return err
		}
		defer r.Body.Close()
	}

	return nil
}

func unloadRequestBodyGenerator(modelName string) (*bytes.Buffer, error) {
	data := map[string]any{
		"model":      modelName,
		"keep_alive": 0,
	}

	out, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(out), nil
}

func main() {
	if err := RootCommand.Execute(); err != nil {
		log.Fatalln(err)
	}
}
