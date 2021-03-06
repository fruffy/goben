package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const version = "0.4"

type hostList []string

type config struct {
	hosts          hostList
	listeners      hostList
	defaultPort    string
	connections    int
	reportInterval string
	totalDuration  string
	opt            options
	passiveClient  bool // suppress client send
	udp            bool
	chart          string
	export         string
	csv            string
	silent         bool // suppress any output
	tlsCert        string
	tlsKey         string
	tls            bool
}

func (h *hostList) String() string {
	return fmt.Sprint(*h)
}

func (h *hostList) Set(value string) error {
	for _, hh := range strings.Split(value, ",") {
		*h = append(*h, hh)
	}
	return nil
}

func badExportFilename(parameter, filename string) error {
	if filename == "" {
		return nil
	}

	if strings.Contains(filename, "%d") && strings.Contains(filename, "%s") {
		return nil
	}

	return fmt.Errorf("badExportFilename %s: filename requires '%%d' and '%%s': %s", parameter, filename)
}

func main() {

	app := config{}

	flag.Var(&app.hosts, "hosts", "comma-separated list of hosts\nyou may append an optional port to every host: host[:port]")
	flag.Var(&app.listeners, "listeners", "comma-separated list of listen addresses\nyou may prepend an optional host to every port: [host]:port")
	flag.StringVar(&app.defaultPort, "defaultPort", ":8080", "default port")
	flag.IntVar(&app.connections, "connections", 1, "number of parallel connections")
	flag.StringVar(&app.reportInterval, "reportInterval", "2s", "periodic report interval\nunspecified time unit defaults to second")
	flag.StringVar(&app.totalDuration, "totalDuration", "10s", "test total duration\nunspecified time unit defaults to second\ninf means unlimited time, and when it's set to inf, no ascii chart will be rendered")
	flag.IntVar(&app.opt.ReadSize, "readSize", 50000, "read buffer size in bytes")
	flag.IntVar(&app.opt.WriteSize, "writeSize", 50000, "write buffer size in bytes")
	flag.BoolVar(&app.passiveClient, "passiveClient", false, "suppress client writes")
	flag.BoolVar(&app.opt.PassiveServer, "passiveServer", false, "suppress server writes")
	flag.Float64Var(&app.opt.MaxSpeed, "maxSpeed", 0, "bandwidth limit in mbps (0 means unlimited)")
	flag.BoolVar(&app.udp, "udp", false, "run client in UDP mode")
	flag.StringVar(&app.export, "export", "", "output filename for YAML exporting test results on client\n'%d' is parallel connection index to host\n'%s' is hostname:port\nexample: -export export-%d-%s.yaml")
	flag.StringVar(&app.csv, "csv", "", "output filename for CSV exporting test results on client\n'%d' is parallel connection index to host\n'%s' is hostname:port\nexample: -csv export-%d-%s.csv")
	flag.BoolVar(&app.silent, "silent", false, "Do not print any output")
	flag.StringVar(&app.tlsKey, "key", "key.pem", "TLS key file")
	flag.StringVar(&app.tlsCert, "cert", "cert.pem", "TLS cert file")
	flag.BoolVar(&app.tls, "tls", false, "set to false to disable TLS")
	flag.Uint64Var(&app.opt.totalFlow, "totalFlow", 0, "test bandwidth/latency by given total amount of data transmitted over each connection\ndata unit defaults to kB, totalDuration flag will be disabled")

	flag.Parse()
	if (app.silent) {
		log.SetOutput(ioutil.Discard)
	}

	if errExport := badExportFilename("-export", app.export); errExport != nil {
		log.Panicf("%s", errExport.Error())
	}

	if errCsv := badExportFilename("-csv", app.csv); errCsv != nil {
		log.Panicf("%s", errCsv.Error())
	}

	app.reportInterval = defaultTimeUnit(app.reportInterval)

	app.totalDuration = defaultTimeUnit(app.totalDuration)

	var errInterval error
	app.opt.ReportInterval, errInterval = time.ParseDuration(app.reportInterval)
	if errInterval != nil {
		log.Panicf("bad reportInterval: %q: %v", app.reportInterval, errInterval)
	}

	var errDuration error
	app.opt.TotalDuration, errDuration = time.ParseDuration(app.totalDuration)
	if errDuration != nil {
		log.Panicf("bad totalDuration: %q: %v", app.totalDuration, errDuration)
	}

	if len(app.listeners) == 0 {
		app.listeners = []string{app.defaultPort}
	}

	log.Printf("goben version " + version + " runtime " + runtime.Version() + " GOMAXPROCS=" + strconv.Itoa(runtime.GOMAXPROCS(0)))
	log.Printf("connections=%d defaultPort=%s listeners=%q hosts=%q",
		app.connections, app.defaultPort, app.listeners, app.hosts)
	log.Printf("reportInterval=%s totalDuration=%s", app.opt.ReportInterval, app.opt.TotalDuration)

	if len(app.hosts) == 0 {
		log.Printf("server mode (use -hosts to switch to client mode)")
		serve(&app)
		return
	}

	var proto string
	if app.udp {
		proto = "udp"
	} else {
		proto = "tcp"
	}

	log.Printf("client mode, %s protocol", proto)
	open(&app)
}

// append "s" (second) to time string
func defaultTimeUnit(s string) string {
	if len(s) < 1 {
		return s
	}
	// enable the client and server to generate traffic for an unlimited time duration
	if s == "inf" {
		// the longest possible time duration is 290 years, which is effectively infinite long
		return strconv.FormatInt(math.MaxInt64, 10) + "ns"
	}
	if unicode.IsDigit(rune(s[len(s)-1])) {
		return s + "s"
	}
	return s
}
