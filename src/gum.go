package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

// Config file name (will add path later)
var sConfigFilePath = "configuration.json"

// Global variables
var bDebug bool = true
var eErr error

// Shout out to the following website: https://mholt.github.io/json-to-go/
type Protocol struct {
	Protocol            string     `json:"protocol"`
	CMD                 []string   `json:"cmd"`
	OutputFile          string     `json:"outputfile"`
	BaseFile            string     `json:"basefile"`
	FileDelimeter       string     `json:"filedelimeter"`
	HostAddressFileLine []string   `json:"hostaddressfileline"`
	UsernameFileLine    []string   `json:"usernamefileline"`
	AdditionalFileLines [][]string `json:"additionalfilelines"`
}

type AppConfig struct {
	Debug     bool       `json:"debug"`
	Protocols []Protocol `json:"protocols"`
}

type FragmentData [][]string

type TemplateVariables struct {
	FullURL     string
	Protocol    string
	HostAddress string
	HostPort    string
	Username    string
	Path        string
	OutputFile  string
	Query       url.Values
}

type SettingPair struct {
	Setting string
	Value   string
}

func main() {

	defer func() {
		if r := recover(); r != nil {
			time.Sleep(4 * time.Second)
		}
	}()

	setConfigFilePath()

	if len(os.Args) > 1 {
		program()
	} else {
		help()
	}

	if bDebug {
		time.Sleep(4 * time.Second)
	}
}

func setConfigFilePath() {
	// Get the path of our binary
	sBinaryPath, _ := os.Executable()

	// Get our path separator
	sPathSeparator := string(os.PathSeparator)

	// Break path into string array
	aPath := strings.Split(sBinaryPath, sPathSeparator)

	// Drop the last element of the array
	aPath = aPath[0:(len(aPath) - 1)]

	// Rebuild our config file path
	sConfigFilePath = strings.Join(aPath, sPathSeparator) + sPathSeparator + sConfigFilePath
}

func help() {
	// Version format is YYMMDDBB (Year. month, day, build)
	fmt.Printf("Golang URL Middleware" + "\n" +
		"Version: 24020101" + "\n" +
		"Author: Nathan Jackson" + "\n" +
		"GitHub: https://github.com/nathancrjackson/gum-go" + "\n" +
		"\n" +
		"Usage:" + "\n" +
		"  gum [options] <URL>" + "\n" +
		"\n" +
		"Options:" + "\n" +
		"  [CURRENTLY ARE NONE]" + "\n\n")

	bCanLoadConf, acData := loadConfig(sConfigFilePath)

	if bCanLoadConf {
		fmt.Printf("Can load configuration file.\n\n")

		if len(acData.Protocols) > 0 {
			fmt.Println("Configured protocols are:")
			for i := 0; i < len(acData.Protocols); i++ {
				fmt.Println("- " + acData.Protocols[i].Protocol)
			}
		} else {
			fmt.Println("No protocols configured.")
		}

	} else {
		fmt.Println("Cannot load configuration file: ", sConfigFilePath)
	}

}

func program() {
	//Initiate some variables
	var bCont = true
	var acData AppConfig
	var pConfig Protocol
	var tvData TemplateVariables
	var sURLPath string
	var sUsername string
	var sHostAddr string
	var sHostPort string
	var sFragment string

	//Check input args for URL
	if len(os.Args) == 2 {
		sURLPath = os.Args[1]
	} else {
		fmt.Printf("Too many arguments.\n\n")
		help()
		return
	}

	//Prep our template variables struct and load in our first bit of data
	tvData = TemplateVariables{FullURL: sURLPath}

	debugPrint("Loading our app configuration")
	bCont, acData = loadConfig(sConfigFilePath)
	if !bCont {
		debugPrint("Exiting early")
		return
	}

	//Track if we are panicking where there is trouble
	bDebug = acData.Debug

	//Output target to console
	fmt.Println("Connecting to:", tvData.FullURL)

	debugPrint("Take in a URL and parse it")
	u, eErr := url.Parse(sURLPath)
	if handleError(eErr, "Could not parse URL:", sURLPath) {
		return
	}

	debugPrint("Checking for matching protocol")
	for i := 0; i < len(acData.Protocols); i++ {
		//Loop through protocol configurations to see if one matches
		sProtocol := &acData.Protocols[i]
		debugPrint("- " + sProtocol.Protocol)
		if strings.EqualFold(sProtocol.Protocol, u.Scheme) {
			pConfig = acData.Protocols[i]
		}
	}

	//If not gracefully exit
	if pConfig.Protocol == "" {
		fmt.Println("App config does not support:", u.Scheme)
		return
	}

	debugPrint("Storing the hostname and port")
	if strings.Contains(u.Host, ":") {
		sHostAddr, sHostPort, eErr = net.SplitHostPort(u.Host)
		if handleError(eErr, "Could not parse host:", u.Host) {
			return
		}
	} else {
		sHostAddr = u.Host
	}

	debugPrint("Processing our username if available")
	if u.User != nil {
		sUsername, eErr = url.QueryUnescape(u.User.String())
		if handleError(eErr, "Could not parse username:", u.User.String()) {
			return
		}
	}
	if sUsername != "" {
		fmt.Println("User:", sUsername)
	} else {
		fmt.Println("No user specified")
	}

	debugPrint("Load data into our Template variables")
	tvData.Protocol = pConfig.Protocol
	tvData.HostAddress = sHostAddr
	tvData.HostPort = sHostPort
	tvData.Username = sUsername
	tvData.Path = u.Path
	tvData.Query = u.Query()
	tvData.OutputFile = pConfig.OutputFile

	//Start creating an output if there is one set
	if pConfig.OutputFile != "" {
		debugPrint("Creating output file")

		//Prepare our output file contents
		mFileContents := make(map[string][]string)

		//Load in our basefile if exists
		if pConfig.BaseFile != "" {
			debugPrint("Loading base file")
			bCont, mFileContents = loadBaseFile(pConfig.BaseFile, pConfig.FileDelimeter)
			if !bCont {
				debugPrint("Exiting early")
				return
			}
		}

		//Process our fragment if it's not empty
		if u.Fragment != "" {
			debugPrint("Processing URL fragment")
			sFragment = u.Fragment

			//Decode the fragment
			baDecoded, eErr := base64.StdEncoding.DecodeString(sFragment)
			if handleError(eErr, "Could not base64 decode fragment:", sFragment) {
				return
			}

			//Convert byte array to string
			sDecoded := string(baDecoded)

			//Decode resulting JSON
			var fdJSON FragmentData
			if handleError(eErr, "Could not json decode:", sDecoded) {
				return
			}

			debugPrint("Merging the fragment data into our file contents")
			for i := 0; i < len(fdJSON); i++ {
				mFileContents[fdJSON[i][0]] = fdJSON[i][1:]
			}
		}

		//Load in extra lines if defined in the config
		if len(pConfig.AdditionalFileLines) > 0 {
			debugPrint("Merging additional lines")
			//For each additional line
			for i := 0; i < len(pConfig.AdditionalFileLines); i++ {
				bCont, aAdditionalStrings := parseTemplateStringArray(pConfig.AdditionalFileLines[i], tvData)
				mFileContents[aAdditionalStrings[0]] = aAdditionalStrings[1:]
				if !bCont {
					debugPrint("Exiting early")
					return
				}
			}
		}
		if len(pConfig.HostAddressFileLine) > 0 {
			debugPrint("Merging host address line")
			bCont, aHostStrings := parseTemplateStringArray(pConfig.HostAddressFileLine, tvData)
			mFileContents[aHostStrings[0]] = aHostStrings[1:]
			if !bCont {
				debugPrint("Exiting early")
				return
			}
		}
		if len(pConfig.UsernameFileLine) > 0 {
			debugPrint("Merging username line")
			bCont, aUserStrings := parseTemplateStringArray(pConfig.UsernameFileLine, tvData)
			mFileContents[aUserStrings[0]] = aUserStrings[1:]
			if !bCont {
				debugPrint("Exiting early")
				return
			}
		}

		debugPrint("Creating our output file")
		createOutputFile(pConfig.OutputFile, pConfig.FileDelimeter, mFileContents)
	}

	debugPrint("Parsing templates in our cmd array")
	bCont, pConfig.CMD = parseTemplateStringArray(pConfig.CMD, tvData)
	if !bCont {
		debugPrint("Exiting early")
		return
	}

	debugPrint("Executing our cmd array")
	aArgs := pConfig.CMD[1:]
	cmd := exec.Command(pConfig.CMD[0], aArgs...)
	eErr = cmd.Start()
	if handleError(eErr, "Could not start:", pConfig.CMD[0]) {
		return
	}
	debugPrint("Done program")
}

func handleError(eErr error, messages ...string) bool {
	if eErr != nil {
		fmt.Println(messages)
		if bDebug {
			fmt.Println("----- Error String -----")
			panic(eErr)
		}
		return true
	}
	return false
}

func debugPrint(message string) {
	if bDebug {
		fmt.Println(message)
	}
}

func loadConfig(filepath string) (bool, AppConfig) {
	var result AppConfig

	// Print path for reference
	debugPrint("Config file path is: " + sConfigFilePath)

	filecontents, eErr := os.ReadFile(filepath)
	if handleError(eErr, "Could not load:", filepath) {
		return false, result
	}

	eErr = json.Unmarshal(filecontents, &result)
	if handleError(eErr, "Could not process:", filepath) {
		return false, result
	}

	return true, result
}

func loadBaseFile(sFilePath string, sDilimeter string) (bool, map[string][]string) {
	result := make(map[string][]string)

	//Open our file
	file, eErr := os.Open(sFilePath)
	defer file.Close()
	if handleError(eErr, "Could not load:", sFilePath) {
		return false, result
	}

	//This may error if lines are longer than lines longer than 65536 characters
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		sArray := strings.Split(scanner.Text(), sDilimeter)
		result[sArray[0]] = sArray[1:]
	}

	return true, result
}

func parseTemplateString(templateString string, templateData TemplateVariables) (bool, string) {
	//Prepare a byte buffer to output template result into
	var bytebuffer bytes.Buffer

	//Prepare our template
	template, eErr := template.New("string.template").Parse(templateString)
	if handleError(eErr, "Could parse template string:", templateString) {
		return false, templateString
	}

	//Execute our template
	eErr = template.Execute(&bytebuffer, templateData)
	if handleError(eErr, "Could execute template string:", templateString) {
		return false, templateString
	}

	//Return our processed string
	return true, bytebuffer.String()
}

func parseTemplateStringArray(templateStrings []string, templateData TemplateVariables) (bool, []string) {
	bNoFailure := true
	//Loop through array updating values and return it
	for i := 0; i < len(templateStrings); i++ {
		bSuccess := true
		bSuccess, templateStrings[i] = parseTemplateString(templateStrings[i], templateData)
		if !bSuccess {
			bNoFailure = false
		}
	}

	return bNoFailure, templateStrings
}

func createOutputFile(sFileOutputPath string, sArrayDelimeter string, mFileContents map[string][]string) bool {

	file, eErr := os.Create(sFileOutputPath)
	defer file.Close()
	if handleError(eErr, "Could not create:", sFileOutputPath) {
		return false
	}

	sFileContents := ""

	//Loop through array joining sub-arrays and putting into file contents
	for setting, value := range mFileContents {
		//This is making setting it's own string array, then appending all the values in the value array
		value = append([]string{setting}, value...)
		sFileContents = sFileContents + strings.Join(value, sArrayDelimeter) + "\n"
	}

	_, eErr = file.WriteString(sFileContents)
	if handleError(eErr, "Could not write to:", sFileOutputPath) {
		return false
	}
	return true
}

func dumpToJSON(data interface{}) bool {
	output, eErr := json.MarshalIndent(data, "", "  ")
	if handleError(eErr, "Error dumping data interface as JSON") {
		return false
	}
	fmt.Print(string(output))
	return true
}
