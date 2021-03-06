package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type Detail struct {
	Text string `json:"description"`
}

type Coord struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Temperature struct {
	Max float64 `json:"max"`
	Min float64 `json:"min"`
}

type List struct {
	Time        int64       `json:"dt"`
	Pressure    float64     `json:"pressure"`
	Humidity    float64     `json:"humidity"`
	Speed       float64     `json:"speed"`
	Degree      float64     `json:"deg"`
	Clouds      float64     `json:"clouds"`
	Rain        float64     `json:"rain"`
	Details     []Detail    `json:"weather"`
	Temperature Temperature `json:"temp"`
}

type City struct {
	Name  string `json:"name"`
	Coord Coord  `json:"coord"`
}

type Weather struct {
	City  City   `json:"city"`
	Lists []List `json:"list"`
}

func formatDate(unixTimeStamp int64) string {
	ti := time.Unix(unixTimeStamp, 0)
	return fmt.Sprintf("%d-%02d-%02d (%s)",
		ti.Year(), ti.Month(), ti.Day(), ti.Weekday().String()[0:3])
}

// ssh/terminal/util.go GetSize()
func getTerminalSize() (width, height int, err error) {
	var dimensions [4]uint16
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&dimensions)),
		0, 0, 0); err != 0 {
		return -1, -1, err
	}
	return int(dimensions[1]), int(dimensions[0]), nil
}

func printHorizontalLine(width int) {
	fmt.Println(strings.Repeat("–", width))
}

func toCelsius(degree float64) float64 {
	return degree - 273.15
}

func toTitleCase(input string) string {
	return string(bytes.Title([]byte(input)))
}

// http://climate.umn.edu/snow_fence/components/winddirectionanddegreeswithouttable3.htm
func toWindDirection(degree float64) string {
	degree = math.Mod(degree, 360)
	if degree < 0 {
		degree = 360 + degree
	}
	switch {
	case degree >= 11.25 && degree < 33.75:
		return "NNE"
	case degree >= 33.75 && degree < 56.25:
		return "NE"
	case degree >= 56.25 && degree < 78.75:
		return "ENE"
	case degree >= 78.75 && degree < 101.25:
		return "E"
	case degree >= 101.25 && degree < 123.75:
		return "ESE"
	case degree >= 123.75 && degree < 146.25:
		return "SE"
	case degree >= 146.25 && degree < 168.75:
		return "SSE"
	case degree >= 168.75 && degree < 191.25:
		return "S"
	case degree >= 191.25 && degree < 213.75:
		return "SSW"
	case degree >= 213.75 && degree < 236.25:
		return "SW"
	case degree >= 236.25 && degree < 258.75:
		return "WSW"
	case degree >= 258.75 && degree < 281.25:
		return "W"
	case degree >= 281.25 && degree < 303.75:
		return "WNW"
	case degree >= 303.75 && degree < 326.25:
		return "NW"
	case degree >= 326.25 && degree < 348.75:
		return "NNW"
	default:
		return "N"
	}
}

func main() {
	city := flag.String("city", "Daliang", "<name>     Name of the city")
	fake := flag.Bool("fake", false, "           Use fake data")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTION] [[of] city]\n\n", os.Args[0])
		flag.VisitAll(func(flag *flag.Flag) {
			switch flag.DefValue {
			case "true", "false":
				fmt.Fprintf(os.Stderr, "  --%s %s\n", flag.Name, flag.Usage)
			default:
				fmt.Fprintf(os.Stderr, "  --%s %s, default is %s\n",
					flag.Name, flag.Usage, flag.DefValue)
			}
		})
	}
	flag.Parse()

	rest := flag.NArg()
	if rest > 0 {
		start := 0
		if flag.Arg(0) == "of" {
			start++
		}
		args := flag.Args()
		*city = strings.Join(args[start:len(args)], " ")
	}

	termWidth, _, _ := getTerminalSize()

	pgfmt := [9]string{
		"  Time      %s",
		"  Status    %s",
		"  Clouds    %s",
		"  Degree    %s",
		"  Humidity  %s",
		"  Pressure  %s",
		"  Rain      %s",
		"  Speed     %s",
		"  Temp (C)  %s",
	}

	api := "http://api.openweathermap.org/data/2.5/forecast/daily?q=%s"

	var res *http.Response
	var body []byte
	var err error

	if !*fake {
		res, err = http.Get(fmt.Sprintf(api, *city))
	}
	if !*fake && err == nil {
		body, err = ioutil.ReadAll(res.Body)
		defer res.Body.Close()
	} else {
		body = []byte(FAKE_DATA)
	}

	if err != nil {
		panic(err)
	}

	weather := &Weather{}
	json.Unmarshal(body, &weather)

	if weather.City.Name == "" {
		fmt.Fprintf(os.Stderr, "No weather forecast for city %s!\n", *city)
		os.Exit(1)
	}

	title := fmt.Sprintf("Weather Forecast for %s (%0.5f, %0.5f)",
		weather.City.Name,
		weather.City.Coord.Lat,
		weather.City.Coord.Lon)

	offset := int(math.Max(math.Floor(float64(termWidth-len(title))/2), 0))

	printHorizontalLine(termWidth)
	fmt.Printf("%*s%s\n", offset, "", title)
	printHorizontalLine(termWidth)

	listLen := len(weather.Lists)

	colWidth := 34.0
	cols := int(math.Max(math.Floor(float64(termWidth)/colWidth), 1.0))
	rows := int(math.Ceil(float64(listLen) / float64(cols)))

	data := make([][9]string, listLen)

	for i := range weather.Lists {
		list := weather.Lists[i]
		data[i] = [9]string{
			formatDate(list.Time),
			toTitleCase(list.Details[0].Text),
			fmt.Sprintf("%.0f%%", list.Clouds),
			fmt.Sprintf("%.0f (%s)", list.Degree, toWindDirection(list.Degree)),
			fmt.Sprintf("%.0f%%", list.Humidity),
			fmt.Sprintf("%.2f hPa", list.Pressure),
			fmt.Sprintf("%.0f mm", list.Rain),
			fmt.Sprintf("%.2f mps", list.Speed),
			fmt.Sprintf("%.2f Hi / %.2f Lo",
				toCelsius(list.Temperature.Max),
				toCelsius(list.Temperature.Min)),
		}
	}

	for i := 0; i < rows; i++ {
		for j := 0; j < len(pgfmt); j++ {
			for k := 0; k < cols; k++ {
				index := i*cols + k
				if index >= listLen {
					continue
				}
				o := fmt.Sprintf(pgfmt[j], data[index][j])
				f := int(colWidth) - len(o)
				if f > 0 {
					fmt.Printf("%s%*s", o, f, " ")
				} else {
					fmt.Print(o)
				}
			}
			fmt.Print("\n")
		}
		printHorizontalLine(termWidth)
	}
}
