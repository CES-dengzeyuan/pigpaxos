package main

import (
	"encoding/csv"
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
	file, err := os.Open("ip_list.csv")
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}(file)

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		panic(err)
	}

	ipList := make([]string, 0)
	for i, record := range records {
		if i == 0 {
			continue
		}
		publicIP := record[9]
		ipList = append(ipList, publicIP)
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
			T:                    60,
			N:                    0,
			K:                    1000,
			W:                    0.5,
			Throttle:             0,
			Concurrency:          1,
			Distribution:         "conflict",
			LinearizabilityCheck: false,
			Conflicts:            0,
			Min:                  0,
			Mu:                   500,
			Sigma:                50,
			Move:                 false,
			Speed:                10,
			ZipfianS:             2,
			ZipfianV:             1,
			Lambda:               0.01,
			Size:                 8,
		},
	}

	tcpPort := 1735
	httpPort := 8081

	for i, ip := range ipList {
		key := fmt.Sprintf("1.%d", i+1) // Generate key based on index: "1", "2", "3", ...
		config.Address[key] = fmt.Sprintf("tcp://%s:%d", ip, tcpPort)
		config.HttpAddress[key] = fmt.Sprintf("http://%s:%d", ip, httpPort)
		//tcpPort++
		//httpPort++
	}

	jsonData, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		panic(err)
	}

	// Write JSON to file
	err = os.WriteFile("bin/config.json", jsonData, 0644)
	if err != nil {
		panic(err)
	}

	fmt.Println("config.json file has been written successfully")
}
