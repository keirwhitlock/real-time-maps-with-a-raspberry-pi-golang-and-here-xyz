package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/adrianmo/go-nmea"
	"github.com/jacobsa/go-serial/serial"
	geojson "github.com/paulmach/go.geojson"
)

type HereDev struct {
	Token   string `json:"token"`
	SpaceId string `json:"space_id"`
}

func NewGeoJSON(latitude, longitude float64) ([]byte, error) {

	featureCollection := geojson.NewFeatureCollection()
	feature := geojson.NewPointFeature([]float64{longitude, latitude})
	featureCollection.AddFeature(feature)

	return featureCollection.MarshalJSON()
}

func (here *HereDev) PushToXYZ(data []byte) ([]byte, error) {

	endpoint, _ := url.Parse("https://xyz.api.here.com/hub/spaces/" + here.SpaceId + "/features")
	request, _ := http.NewRequest("PUT", endpoint.String(), bytes.NewBuffer(data))

	request.Header.Set("Content-Type", "application/geo+json")
	request.Header.Set("Authorization", "Bearer "+here.Token)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(response.Body)

}

func DownloadFile(url, filepath string) error {

	// Create file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}

	// Close file at end of func call
	defer out.Close()

	// Download data from url
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write data io to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func main() {

	token := flag.String("token", "", "Here XYZ Token")
	spaceid := flag.String("spaceid", "", "Here XYZ Space ID")
	binaryUrl := flag.String("url", "", "URL for compiled binary")
	filepath := flag.String("filepath", "/usr/local/bin/gps-project", "Filepath to save compiled binary")
	debugEnabled := flag.Bool("debug", false, "Enable debug")

	flag.Parse()

	if *debugEnabled {
		fmt.Printf(".: DEBUG ENABLED :.\n\n")
	}

	if *binaryUrl != "" {
		err := DownloadFile(*binaryUrl, *filepath)
		if err != nil {
			log.Fatalf("file download: %v", err)
		}
	}

	options := serial.OpenOptions{
		PortName:        "/dev/ttyS0",
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	serialPort, err := serial.Open(options)
	if err != nil {
		log.Fatalf("serial.Open: %v", err)
	}

	defer serialPort.Close()

	reader := bufio.NewReader(serialPort)

	scanner := bufio.NewScanner(reader)

	here := HereDev{
		Token:   *token,
		SpaceId: *spaceid,
	}

	for scanner.Scan() {
		if *debugEnabled {
			fmt.Println(scanner.Text())
		}
		sentence, err := nmea.Parse(scanner.Text())
		if err != nil {
			log.Fatalf("nmea.Parse: %v", err)
		}

		if sentence.Prefix() == nmea.PrefixGPRMC {
			data := sentence.(nmea.GPRMC)
			if data.Latitude != 0 && data.Longitude != 0 {
				gjson, err := NewGeoJSON(data.Latitude, data.Longitude)
				if err != nil {
					log.Fatal(err)
				}
				response, err := here.PushToXYZ(gjson)
				if err != nil {
					log.Fatal(err)
				}
				if *debugEnabled {
					fmt.Printf("PUT Response: \n\n%v", response)
				}
			}
		}
	}
}
