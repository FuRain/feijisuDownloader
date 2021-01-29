package hlss

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"feijisu/downloader"
	"feijisu/utils"
	"github.com/evilsocket/islazy/log"

    "io"
    go_log "log"
    "runtime/debug"
    "sync"
)

type DecryptCallback func(string, int, int)

// const defaultChanCache = 1000

type Hlss struct {
	baseUrl          string
	key              []byte
	iv               []byte
	mainIdx          string
	secondaryIdx     []string
	segments         []string
	file             string
	pout             *os.File
	resolutions      map[string]string
	resKeys          []string
	secondaryUrl     string
	bandwidths       map[string]string
	bandwidthKeys    []string
	downloadCallback downloader.Callback
	decryptCallback  DecryptCallback
	downloadWorker   int
	cookies          []*http.Cookie
	referer          string

    fileDir string
    FileAndPath string
    // syncChan chan string
    // wg sync.WaitGroup
}

func New(mainUrl string, key []byte, outputfile string, downloadCallback downloader.Callback, decryptCallback DecryptCallback, downloadWorker int, cookieFile string, referer string, keyUrl string, tsPath string) (*Hlss, error) {
	obj := Hlss{
		mainIdx:          mainUrl,
		key:              key,
		file:             outputfile,
		downloadCallback: downloadCallback,
		decryptCallback:  decryptCallback,
		downloadWorker:   downloadWorker,
		referer:          referer,
        fileDir: tsPath,
		FileAndPath: outputfile,
	}

	if cookieFile != "" {
		log.Debug("parsing cookies")
		err := obj.setCookies(cookieFile)
		if err != nil {
			return nil, err
		}
	}

	// Try to get key from URL
	if keyUrl != "" {
		log.Debug("getting key from url: '%s'", keyUrl)
		resp, err := utils.HttpRequest("GET", keyUrl, obj.cookies, obj.referer)
		if err != nil {
			return nil, fmt.Errorf("http request error: %s", err)
		}
		defer resp.Body.Close()
		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		obj.key = buf
	}

	obj.resolutions = make(map[string]string)
	if err := obj.parseMainIndex(); err != nil {
		return nil, err
	}

	obj.baseUrl = utils.GetBaseUrl(mainUrl)
	log.Debug("base url: '%s'", obj.baseUrl)

	return &obj, nil
}

func (h *Hlss) parseMainIndex() error {
	resp, err := utils.HttpRequest("GET", h.mainIdx, h.cookies, h.referer)
	if err != nil {
		return fmt.Errorf("http request error: %s", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	var currentResolution string
	var resolutionKeys []string
	var currentBandwidth string
	firstLine := true

	for scanner.Scan() {
		line := scanner.Text()
		if firstLine && !strings.HasPrefix(line, "#EXTM3U") {
			return errors.New("parseMainIndex: Invalid m3u file format")
		} else {
			firstLine = false
		}

		if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			line = line[len("#EXT-X-STREAM-INF:"):]

			params := strings.Split(line, ",")
			if len(params) < 2 {
				return errors.New("Invalid m3u file format")
			}
			for _, info := range params {
				if strings.HasPrefix(info, "BANDWIDTH=") {
					currentBandwidth = info[len("BANDWIDTH="):]
				} else if strings.HasPrefix(info, "RESOLUTION=") {
					currentResolution = info[len("RESOLUTION="):]
				}
			}
		} else if strings.HasPrefix(line, "#") || line == "" {
			continue
		} else if currentBandwidth != "" || currentResolution != "" {
			currentTrack := currentBandwidth
			if currentResolution != "" {
				currentTrack = "[" + currentResolution + "] " + currentTrack
			}
			resolutionKeys = append(resolutionKeys, currentTrack)
			h.resolutions[currentTrack] = scanner.Text()
			currentResolution = ""
			currentBandwidth = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	h.resKeys = resolutionKeys

	return nil
}

func (h *Hlss) parseSecondaryIndex() error {
	resp, err := utils.HttpRequest("GET", h.secondaryUrl, h.cookies, h.referer)
	if err != nil {
		return fmt.Errorf("http request error: %s", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	baseUrl := utils.GetBaseUrl(h.secondaryUrl)

	firstLine := true
	getSegment := false
	//keyUrl := ""
	iv := ""
	for scanner.Scan() {
		line := scanner.Text()
		if firstLine && !strings.HasPrefix(line, "#EXTM3U") {
			return errors.New("parseSecondaryIndex: Invalid m3u file format")
		} else {
			firstLine = false
		}

		if strings.HasPrefix(line, "#EXTINF") {
			getSegment = true
		} else if strings.HasPrefix(line, "#EXT-X-KEY:") {
			line = line[len("#EXT-X-KEY:"):]

			params := strings.Split(line, ",")
			if len(params) < 2 {
				return errors.New("Invalid m3u file format")
			}
			for _, info := range params {
				if strings.HasPrefix(info, "URI=\"") {
					//keyUrl = info[len("URI=\"") : len(info)-1]
				} else if strings.HasPrefix(info, "IV=") {
					iv = info[len("IV="):]
					log.Debug("IV found: %s", iv)
					h.iv, err = hex.DecodeString(iv[2:])
					if err != nil {
						return err
					}
				}
			}
		} else if strings.HasPrefix(line, "#") || line == "" {
			continue
		} else if getSegment {
			if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
				h.segments = append(h.segments, line)
			} else {
				h.segments = append(h.segments, baseUrl+line)
			}
			getSegment = false
		}
	}
	if e := scanner.Err(); e != nil {
		return e
	}

	return nil
}

func (h *Hlss) downloadSegments() error {
	log.Debug("downloading segments")
	// d := downloader.New(h.downloadWorker, ".", h.downloadCallback)
	d := downloader.New(h.downloadWorker, h.fileDir, h.downloadCallback)
	d.SetUrls(h.segments, h.file)
	d.SetCookies(h.cookies)
	d.SetReferer(h.referer)
	d.StartDownload()

	return nil
}

const ffmpegPrefix string = "concat:"
func (h *Hlss) decryptSegments() error {
	log.Debug("decrypting segments")
	pout, err := os.Create(h.file)
	defer pout.Close()
	if err != nil {
		return err
	}

	n := 0
	for _, url := range h.segments {
		name := utils.GetFileFromUrl(url)
        name = h.fileDir + name

		if len(h.key) != 0 {
			if err = utils.DecryptFileAppend(pout, name, h.key, h.iv); err != nil {
				return err
			}
		} else {
			// we assume that the segments are not encrypted
			if err = utils.FileAppend(pout, name); err != nil {
				return err
			}
		}

		os.Remove(name)
		n++

		if h.decryptCallback != nil {
			h.decryptCallback(name, n, h.GetTotSegments())
		}
	}

	return nil
}

//! Public methods

func (h *Hlss) ExtractVideo() error {
	var err error
	if h.secondaryUrl == "" {
		h.secondaryUrl = h.mainIdx
		if err = h.parseSecondaryIndex(); err != nil {
			return err
		}
	}

	if err = h.downloadSegments(); err != nil {
		return err
	}

	// if err = h.decryptSegments(); err != nil {
	// 	return err
	// }

	return nil
}

func (h *Hlss) GetResolutions() []string {
	return h.resKeys
}

func (h *Hlss) SetResolution(res_idx int) error {
	if res_idx >= len(h.resKeys) {
		return errors.New("Resolution not found")
	}

	if strings.HasPrefix(h.resolutions[h.resKeys[res_idx]], "http://") || strings.HasPrefix(h.resolutions[h.resKeys[res_idx]], "https://") {
		h.secondaryUrl = h.resolutions[h.resKeys[res_idx]]
	} else {
		h.secondaryUrl = h.baseUrl + h.resolutions[h.resKeys[res_idx]]
	}

	err := h.parseSecondaryIndex()

	return err
}

func (h *Hlss) GetTotSegments() int {
	return len(h.segments)
}

func (h *Hlss) GetBandwidths() []string {
	return h.bandwidthKeys
}

func (h *Hlss) setCookies(cookieFile string) error {
	if cookieFile != "" {
		cookies, err := utils.ParseCookieFile(cookieFile)
		if err != nil {
			return fmt.Errorf("cannot parse cookie file: %s", err)
		}
		h.cookies = cookies
	}
	return nil
}

func (h *Hlss) AppendMerge() error {
    var err error
	if err = h.decryptSegments(); err != nil {
		return err
	}
    return nil
}

func (h *Hlss) FFMerge() error {
    segTsArr := make([]string, len(h.segments))
    for index, url := range h.segments {
        name := utils.GetFileFromUrl(url)
        name = h.fileDir + name
        segTsArr[index] = name
    }

    concatStr := strings.Join(segTsArr, "|")
    cmd := exec.Command("ffmpeg", "-i", ffmpegPrefix + concatStr, h.file)
    // log.Printf("Running command and waiting for it to finish...")
    err := cmd.Run()
    if err != nil {
        go_log.Printf("Command finished with error: %v\n", err)
    }
    // delete .ts file.
    for _, url := range h.segments {
        name := utils.GetFileFromUrl(url)
        name = h.fileDir + name
        os.Remove(name)
    }

    return err
}

func readLog(wg *sync.WaitGroup, out chan string, reader io.ReadCloser) {
    defer func() {
        if r := recover(); r != nil {
            go_log.Println(r, string(debug.Stack()))
        }
    }()
    defer wg.Done()
    r := bufio.NewReader(reader)
    for {
        line, _, err := r.ReadLine()
        if err == io.EOF || err != nil {
            return
        }
        out <- string(line)
    }
}

// RunCommand run shell
func RunCommand(out chan string, name string, arg ...string) error {
    cmd := exec.Command(name, arg...)
    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()
    if err := cmd.Start(); err != nil {
        return err
    }
    wg := sync.WaitGroup{}
    defer wg.Wait()
    wg.Add(2)
    go readLog(&wg, out, stdout)
    go readLog(&wg, out, stderr)
    if err := cmd.Wait(); err != nil {
        return err
    }
    return nil
}
