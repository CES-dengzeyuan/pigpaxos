package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Address        map[string]string `json:"address"`
	HttpAddress    map[string]string `json:"http_address"`
	Policy         string            `json:"policy"`
	Threshold      int               `json:"threshold"`
	Thrifty        bool              `json:"thrifty"`
	ChanBufferSize int               `json:"chan_buffer_size"`
	BufferSize     int               `json:"buffer_size"`
	Multiversion   bool              `json:"multiversion"`
	UseRetroLog    bool              `json:"use_retro_log"`
	Benchmark      Benchmark         `json:"benchmark"`
}

type Benchmark struct {
	T                    int     `json:"T"`
	N                    int     `json:"N"`
	K                    int     `json:"K"`
	W                    float64 `json:"W"`
	Throttle             int     `json:"Throttle"`
	Concurrency          int     `json:"Concurrency"`
	Distribution         string  `json:"Distribution"`
	LinearizabilityCheck bool    `json:"LinearizabilityCheck"`
	Conflicts            int     `json:"Conflicts"`
	Min                  int     `json:"Min"`
	Mu                   int     `json:"Mu"`
	Sigma                int     `json:"Sigma"`
	Move                 bool    `json:"Move"`
	Speed                int     `json:"Speed"`
	ZipfianS             float64 `json:"Zipfian_s"`
	ZipfianV             int     `json:"Zipfian_v"`
	Lambda               float64 `json:"Lambda"`
	Size                 int     `json:"Size"`
}

func main() {
	ipList := []string{
		"202.199.13.86", "202.189.45.14", "202.199.15.47", // Add more IPs as needed
	}
	config := Config{
		Address:        make(map[string]string),
		HttpAddress:    make(map[string]string),
		Policy:         "majority",
		Threshold:      3,
		Thrifty:        false,
		ChanBufferSize: 1024,
		BufferSize:     1024,
		Multiversion:   false,
		UseRetroLog:    false,
		Benchmark: Benchmark{
			T: 2,
			// ... Add the rest of the benchmark settings
		},
	}

	tcpPort := 1735
	httpPort := 8080

	// Generate the address and http_address based on ipList
	for i, ip := range ipList {
		key := fmt.Sprintf("1.%d", i+1) // Generate key based on index: "1", "2", "3", ...
		config.Address[key] = fmt.Sprintf("tcp://%s:%d", ip, tcpPort)
		config.HttpAddress[key] = fmt.Sprintf("http://%s:%d", ip, httpPort)

		// Increment ports for next IP
		tcpPort++
		httpPort++
	}

	// Serialize config to JSON
	file, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		panic(err)
	}

	// Write JSON to file
	err = os.WriteFile("bin/myconfig.json", file, 0644)
	if err != nil {
		panic(err)
	}

	fmt.Println("config.json file has been written successfully")
}
