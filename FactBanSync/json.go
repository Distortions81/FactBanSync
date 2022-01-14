package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
)

//Read list of servers from file
func readServerListFile() {
	file, err := ioutil.ReadFile(serverConfig.ServerListFile)

	//Read server list file if it exists
	if file != nil && !os.IsNotExist(err) {
		err = json.Unmarshal(file, &serverList)

		if err != nil {
			log.Println("Error reading server list file: " + err.Error())
			os.Exit(1)
		}
	} else {
		//Generate empty list
		serverList = serverListData{Version: "0.0.1", ServerList: []serverData{}}

		log.Println("No server list file found, creating new one.")
		writeServerListFile()
	}
}

//Read server config from file
func readConfigFile() {
	//Read server config file
	file, err := ioutil.ReadFile(configPath)

	if file != nil && err == nil {
		err = json.Unmarshal(file, &serverConfig)

		if err != nil {
			log.Println("Error reading config file: " + err.Error())
			os.Exit(1)
		}

		//Let user know further config is required
		if serverConfig.ServerName == "Default" {
			log.Println("Please change ServerName in the config file")
			os.Exit(1)
		}
	} else {
		//Make example config file, with reasonable defaults
		makeDefaultConfigFile()
		fmt.Println("No config file found, generating defaults, saving to " + configPath)
		log.Println("Please change ServerName in the config file!")
		log.Println("Exiting...")
		os.Exit(1)
	}
}

//Make default-value config file as an example starting point
func makeDefaultConfigFile() {
	serverConfig.Version = version
	serverConfig.ListURL = defaultListURL

	serverConfig.ServerName = "Default"
	serverConfig.BanFile = defaultBanFile
	serverConfig.ServerListFile = defaultServerListFile
	serverConfig.LogDir = defaultLogDir
	serverConfig.BanFileDir = defaultBanFileDir

	serverConfig.RunWebServer = false
	serverConfig.WebPort = 8080

	serverConfig.RCONEnabled = false
	serverConfig.LogMonitoring = false
	serverConfig.AutoSubscribe = true
	serverConfig.RequireReason = false

	serverConfig.FetchBansSeconds = defaultFetchBansSeconds
	serverConfig.WatchSeconds = defaultWatchSeconds
	serverConfig.RefreshListMinutes = defaultRefreshListMinutes

	writeConfigFile()
}

//Read the Factorio ban list file locally
func readServerBanList() {

	file, err := os.Open(serverConfig.BanFile)

	if err != nil {
		log.Println(err)
		return
	}

	var bData []banDataType

	data, err := ioutil.ReadAll(file)

	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	var names []string
	err = json.Unmarshal(data, &names)

	if err != nil {
		//Not really an error, just empty array
		//Only needed because Factorio will write some bans as an array for some unknown reason.
	} else {

		for _, name := range names {
			if name != "" {
				bData = append(bData, banDataType{UserName: name})
			}
		}
	}

	var bans []banDataType
	err = json.Unmarshal(data, &bans)

	if err != nil {
		//Ignore, just array of strings
	}

	for _, item := range bans {
		if item.UserName != "" {
			//It also commonly writes this address, and it isn't neeeded
			if item.Address == "0.0.0.0" {
				item.Address = ""
			}
			bData = append(bData, item)
		}
	}

	banData = bData

	log.Println("Read " + fmt.Sprintf("%v", len(bData)) + " bans from banlist")
}

//Write our ban list to the Factorio ban list file (indent)
func writeBanListFile() {
	file, err := os.Create(serverConfig.BanFile)

	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	outbuf := new(bytes.Buffer)
	enc := json.NewEncoder(outbuf)
	enc.SetIndent("", "\t")

	err = enc.Encode(banData)

	if err != nil {
		log.Println("Error encoding ban list file: " + err.Error())
		os.Exit(1)
	}

	//Cache a normal and gzipped version of the ban list, for the webserver
	if serverConfig.RunWebServer {
		cachedBanListLock.Lock()
		cachedBanList = outbuf.Bytes()
		cachedBanListGz = compressGzip(outbuf.Bytes())
		log.Println("Cached reponse: " + fmt.Sprintf("%v", len(cachedBanList)) + " json / " + fmt.Sprintf("%v", len(cachedBanListGz)) + " gz")
		cachedBanListLock.Unlock()
	}

	wrote, err := file.Write(outbuf.Bytes())

	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("Wrote banlist of " + fmt.Sprintf("%v", len(banData)) + " items, " + fmt.Sprintf("%v", wrote) + " bytes")
}

//Write our server list to the server list file (indent)
func writeConfigFile() {
	file, err := os.Create(configPath)

	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	serverConfig.Version = "0.0.1"
	//Add config file comments

	outbuf := new(bytes.Buffer)
	enc := json.NewEncoder(outbuf)
	enc.SetIndent("", "\t")

	err = enc.Encode(serverConfig)

	if err != nil {
		log.Println("Error writing config file: " + err.Error())
		os.Exit(1)
	}

	wrote, err := file.Write(outbuf.Bytes())
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("Wrote config file: " + fmt.Sprintf("%v", wrote) + " bytes")

}

//Write list of servers to file
func writeServerListFile() {
	file, err := os.Create(serverConfig.ServerListFile)

	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	serverList.Version = "0.0.1"
	outbuf := new(bytes.Buffer)
	enc := json.NewEncoder(outbuf)
	enc.SetIndent("", "\t")

	err = enc.Encode(serverList)

	if err != nil {
		log.Println("Error writing server list file: " + err.Error())
	}

	wrote, err := file.Write(outbuf.Bytes())
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	log.Print("Wrote server list file: " + fmt.Sprintf("%v", wrote) + " bytes")
}

//sanitize a string before use in a filename
func FileNameFilter(str string) string {
	alphafilter, _ := regexp.Compile("[^a-zA-Z0-9-_]+")
	str = alphafilter.ReplaceAllString(str, "")
	return str
}

//Gzip compress a byte array
func compressGzip(data []byte) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write(data)
	w.Close()
	return b.Bytes()
}
