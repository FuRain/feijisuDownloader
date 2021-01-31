package main

import (
	"github.com/evilsocket/islazy/log"

    go_log "log"

	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	// "runtime"
	"strconv"
	"strings"
	"sync"
	"time"
    "os/exec"

	"feijisu/hlss"
)

var (
	aesKey      string
	// url         string
	cookieFile  string
	outFile     string
	referer     string
	dwnWorkers  int
	segments    int
	downloaded  int
	decrypted   int
	isSecondary bool
	debugFlag   bool
)

type videoInfo struct {
    link   string
    detail string
}

func decryptCallback(file string, done int, total int) {
	// if decrypted == 0 {
	// 	fmt.Printf("\n")
	// }
	// if decrypted != 0 && log.Level != log.DEBUG {
	// 	fmt.Print("\033[A") // move cursor up
	// }
	decrypted++
	// fmt.Printf("\r[@] Decrypting %d/%d\n", done, total)
}

func downloadCallback(file string, done int, total int) {
	if downloaded != 0 && log.Level != log.DEBUG {
		fmt.Print("\033[A") // move cursor up
	}
	downloaded++
	fmt.Printf("\r[@] Downloading %d/%d\n", done, total)
}

const resourceLink string = "http://t.mtyee.com/ne2/s%s.js"
const webHome string = "http://www.feijisu5.com/"

func verifyLink(realLink string) bool {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   5 * time.Second,
	}
	resp, err := client.Get(realLink)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return true
	}

	return false
}

func getLink(id string) (string, error) {

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf(resourceLink, id), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.116 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Println("the id resource link does not exist. http code :" + strconv.Itoa(resp.StatusCode))
		return "", errors.New("the id resource link does not exist. http code :" + strconv.Itoa(resp.StatusCode))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return "", err
	}
	return string(body), nil
}

func parseLink(link string) ([][]string, error) {
	/*
	   var lianzaijs_1 = 11;
	   var playarr_1 = new Array();
	   var pl_dy = 2;
	   pl_id = 40970;
	   playarr_1[1] = "https://www5.yuboyun111.com/hls/2019/05/20/Iic0dk6i/playlist.m3u8,zd,1";
	   playarr_1[2] = "https://www5.yuboyun111.com/hls/2019/05/20/Lh2FlITj/playlist.m3u8,zd,2";
	   playarr_1[3] = "https://www5.yuboyun111.com/hls/2019/05/20/PcPvhpS1/playlist.m3u8,zd,3";
	   playarr_1[4] = "https://www5.yuboyun111.com/hls/2019/05/20/Mmsljcpx/playlist.m3u8,zd,4";
	   playarr_1[5] = "https://www5.yuboyun111.com/hls/2019/05/20/trM7JKNq/playlist.m3u8,zd,5";
	   playarr_1[6] = "https://www5.yuboyun111.com/hls/2019/05/20/15BZDOCJ/playlist.m3u8,zd,6";
	   playarr_1[7] = "https://www5.yuboyun111.com/hls/2019/05/20/pYC2y4yx/playlist.m3u8,zd,7";
	   playarr_1[8] = "https://www5.yuboyun111.com/hls/2019/05/20/nQSKcfIz/playlist.m3u8,zd,8";
	   playarr_1[9] = "https://www5.yuboyun111.com/hls/2019/05/20/IonzvEvP/playlist.m3u8,zd,9";
	   playarr_1[10] = "https://www5.yuboyun111.com/hls/2019/05/20/67DW6atz/playlist.m3u8,zd,10";
	   playarr_1[11] = "https://www5.yuboyun111.com/hls/2019/05/20/4nTj0t1s/playlist.m3u8,zd,11";
	   lianzaijs_1_ed = 1;
	*/

	// 1 line is number of series.
	// 2 line it can be ignored.
	// 3 line is resource count of sum.
	// 4 line is resource id.
	// middle line is resource link and resource number.
	// last line it can be ignored.
	// repeat the above.

	linkSubStr := strings.Split(link, ";")
	subStrLine := len(linkSubStr) - 1
	if subStrLine < 6 {
		return nil, errors.New("the link data is incorrect!")
	}

	numSer, err := strconv.Atoi(strings.SplitN(linkSubStr[0], "=", 2)[1])
	if err != nil {
		return nil, errors.New("call strconv.Atoi failed, cause: " + linkSubStr[0])
	}

	resourceSum, err := strconv.Atoi(strings.SplitN(linkSubStr[2], "=", 2)[1])
	if err != nil {
		return nil, errors.New("call strconv.Atoi failed, cause: " + linkSubStr[2])
	}

	realLink := make([][]string, resourceSum)
	for i := 0; i < resourceSum; i++ {
		realLink[i] = make([]string, numSer)
	}

	bIndex := 4
	eIndex := bIndex + numSer

	for i := 0; i < resourceSum && bIndex < subStrLine; i++ {
		iFlag := false
		for j := 0; bIndex < eIndex && bIndex < subStrLine; bIndex, j = bIndex+1, j+1 {
			// fmt.Println(i, linkSubStr[bIndex])
			tmpSubStr := strings.SplitN(linkSubStr[bIndex], "=", 2)[1]
			tmpLink := strings.SplitN(strings.ReplaceAll(tmpSubStr, `"`, ""), ",", 2)[0]
			// Special treatment of certain situations.
			if strings.HasSuffix(tmpLink, ".html") {
				j--
				if !iFlag {
					i--
					iFlag = true
				}
				continue
			}
			realLink[i][j] = tmpLink
		}

		bIndex = eIndex + 5
		eIndex = bIndex + numSer
	}

	return realLink, nil
}

func parseLinkV2(link string) ([][]videoInfo, error) {
    var realLink [][]videoInfo
    jsLine := strings.Split(link, ";")
    numLine := len(jsLine) - 1
    i := 0

    for ; i < numLine; i ++ {
        numSer, err := strconv.Atoi(strings.SplitN(jsLine[i], "=", 2)[1])
        if err != nil {
            return nil, errors.New("call strconv.Atoi failed, cause: " + jsLine[i])
        }

        bLink := i + 4
        eLink := bLink + numSer
        var linkArr []videoInfo

        for ; bLink < eLink && bLink < numLine; bLink ++ {
			tmpSubStr := strings.SplitN(jsLine[bLink], "=", 2)[1]
            subData := strings.Split(strings.ReplaceAll(tmpSubStr, `"`, ""), ",")

            tmpLink := subData[0]
            detail := strings.ReplaceAll(subData[len(subData) - 1], "%u", "\\u")
            zhDetail, err := zhToUnicode([]byte(detail))
            if err != nil {
                zhDetail = []byte(detail)
            }
            // fmt.Println(tmpSubStr, "==============", string(zhDetail))
            linkArr = append(linkArr, videoInfo {
                link : tmpLink,
                detail : string(zhDetail),
            })
        }

        if len(linkArr) > 0 {
            realLink = append(realLink, linkArr)
        }
        i = eLink // site in the "lianzaijs_1_ed = 1;"
    }
	return realLink, nil
}

func startDownload(realLink [][]string) error {
	validIndex := -1

	for key, val := range realLink {
		tempFlag := false
		for _, link := range val {
			if strings.HasSuffix(link, ".m3u8") {
				tempFlag = true
			} else {
				tempFlag = false
				break
			}
		}

		if tempFlag {
			if verifyLink(val[0]) {
				validIndex = key
				break
			}
		}
	}

    if resourceIndex != 0 {
        validIndex = resourceIndex
    }

	if validIndex < 0 {
		return errors.New("does not valid resource link.")
	}

	fmt.Println("use link is : ", validIndex)

	err := os.MkdirAll(id, os.ModePerm)
	if err != nil {
		return err
	}

	// urlIndex := 0
	urlCount := len(realLink[validIndex])

    if endID > urlCount {
        return errors.New(fmt.Sprintf("select download index error, cause: out of range. urlCount: %d", urlCount))
    } else if endID < 0 {
        endID = urlCount
    }

    for i := startID - 1; i < endID; i ++ {
	    callFF(realLink[validIndex][i], i+1)
    }
/*
	remainder := urlCount % cpuNumber
	result := urlCount / cpuNumber
	wg = sync.WaitGroup{}
	for i := 0; i < result; i++ {
		wg.Add(cpuNumber)
		for j := 0; j < cpuNumber; j++ {
			go callFF(realLink[validIndex][urlIndex], urlIndex+1)
			urlIndex++
		}
		wg.Wait()
	}

	wg.Add(remainder)
	for i := 0; i < remainder; i++ {
		go callFF(realLink[validIndex][urlIndex], urlIndex+1)
		urlIndex++
	}
	wg.Wait()
*/
	return nil
}

func callFF(url string, num int) {
	// defer wg.Done()
	// go_log.Println("Start downloading Episode ", num)
    var fileName string
    if videoPrefix != "" {
	    fileName = fmt.Sprintf("%s/%s_%02d.mkv", id, videoPrefix, num)
    } else {
	    fileName = fmt.Sprintf("%s/%02d.mkv", id, num)
    }

	// cmd := exec.Command("./ffmpeg", "-i", url, fileName)
	// err := cmd.Run()
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	var binaryKey []byte
	var keyUrl string
	h, err := hlss.New(url, binaryKey, fileName, downloadCallback, decryptCallback, dwnWorkers, cookieFile, referer, keyUrl, id + "/")
	if err != nil {
		log.Error("%s", err)
		return
	}

	if isSecondary == false {
		// resolutions := h.GetResolutions()
		// fmt.Println("Choose resolution/bandwidth:")
		// for i, k := range resolutions {
		// 	fmt.Printf(" %d) %s\n", i, k)
		// }

        i := 0
        // fmt.Printf("default select resolution/bandwidth is %d\n", i)

		// fmt.Print("> ")
		// var i int
		// _, err = fmt.Scanf("%d", &i)
		// if err != nil {
		// 	log.Error("%s", err)
		// 	return
		// }
		if err = h.SetResolution(i); err != nil {
			log.Error("resolution selection: %s", err)
			return
		}
	}

	downloaded = 0
	decrypted = 0
	segments = h.GetTotSegments()

	// log.Info("download is starting...")
	if err = h.ExtractVideo(); err != nil {
		log.Error("%s", err)
	}

    // start merge .ts file.
    if isUseFFmpeg {
        if err = h.TempMerge(); err != nil {
            log.Error("%s", err)
        } else {
            syncChan<-h
        }
    } else {
        if err = h.AppendMerge(); err != nil {
		    log.Error("%s", err)
        }
    }

}

var id string
var cpuNumber int
var startID int
var endID int
var url string
var resourceIndex int
var isUseFFmpeg bool = false

var syncChan chan * hlss.Hlss
var msgChan chan string
var syncWorkNum int = 0
var videoPrefix string

func main() {
	// argument parse.
	flag.StringVar(&id, "id", "", "please input website video id.")
	verbose := flag.Bool("v", false, "show video resource link.")
	flag.IntVar(&startID, "s", 1, "start download index. default is 1.")
	flag.IntVar(&endID, "e", -1, "end download index. default to last.")
	flag.StringVar(&url, "url", "", "download single link.")
	flag.IntVar(&resourceIndex, "resource", 0, "select the available resource link number.")
	flag.StringVar(&videoPrefix, "o", "", "add download video filename prefix.")

	flag.IntVar(&syncWorkNum, "f", 2, "if use ffmepg merge, it set ffmpeg process count.")

	// flag.StringVar(&aesKey, "k", "", "AES key (base64 encoded or http url)")
	// flag.StringVar(&url, "u", "", "Url master m3u8")
	// flag.StringVar(&outFile, "o", "video.mp4", "Output File")
	// flag.IntVar(&dwnWorkers, "w", 16, "Number of workers to download the segments")
	// flag.BoolVar(&isSecondary, "s", false, "If true the url used on -u parameter will be considered as the secondary index url.")
	// flag.StringVar(&cookieFile, "c", "", "File with authentication cookies.")
	// flag.StringVar(&referer, "r", "", "Set the http referer.")
	// flag.BoolVar(&debugFlag, "debug", false, "Enable debug logs.")

	flag.Parse()

    // Check whether ffmpeg can be used.
    _, err := exec.LookPath("ffmpeg")
    if err == nil {
        isUseFFmpeg = true
        fmt.Println("found ffmpeg site, use it merge video file. ffmpeg version need 4.3 or above.")
    }

    dwnWorkers = 16

	// get cpu number.
	// cpuNumber = runtime.NumCPU()
    cpuNumber = 1

    // download single link.
    if url != "" {
        var singleVideo string
        if videoPrefix != "" {
            singleVideo = videoPrefix + ".mkv"
        } else {
            singleVideo = "one.mkv"
        }

        err := singleLinkDownload(url, singleVideo)
        if err != nil {
            go_log.Fatalln(err)
        }
        return
    }

	if id == "" {
		flag.PrintDefaults()
		return
	}

    if endID >= 0  && startID > endID {
        fmt.Println("select download index error, cause: out of range.")
        return
    }

    if startID <= 0 {
        fmt.Println("select download index error, cause: out of range.")
        return
    }


	link, err := getLink(id)
	if err != nil {
		return
	}

	// fmt.Println(link)

	// realLink, err := parseLink(link)
	realLink, err := parseLinkV2(link)
	if err != nil {
		fmt.Println(err)
		return
	}

	// print resource link and exit.
	if *verbose {
		for key, val := range realLink {
			fmt.Println("resource :", key)
			for k, v := range val {
				fmt.Println(k+1, v)
			}
			fmt.Println("==================================================")
		}
		return
	}

    downLink := make([][]string, len(realLink))
    for i := 0; i < len(realLink); i ++ {
        downLink[i] = make([]string, len(realLink[i]))
        for j := 0; j < len(realLink[i]); j ++ {
            downLink[i][j] = realLink[i][j].link
        }
    }

    wg := sync.WaitGroup{}
    if isUseFFmpeg {
        wg.Add(1)
        go func() {
            processFFMerge()
            wg.Done()
        }()
    }

	err = startDownload(downLink)
	if err != nil {
		fmt.Println(err)
		return
	}
    close(syncChan)

    if isUseFFmpeg {
        go_log.Println("wait all ffmpeg merge file finish.")
    }

    for val := range msgChan {
        go_log.Println(val)
    }

    wg.Wait()

	return
}

func zhToUnicode(raw []byte) ([]byte, error) {
    str, err := strconv.Unquote(strings.Replace(strconv.Quote(string(raw)), `\\u`, `\u`, -1))
    if err != nil {
        return nil, err
    }
    return []byte(str), nil
}

func singleLinkDownload(url string, fileName string) error {
	err := os.MkdirAll("videoDown", os.ModePerm)
	if err != nil {
		return err
	}
	var binaryKey []byte
	var keyUrl string
	h, err := hlss.New(url, binaryKey, "videoDown/" + fileName, downloadCallback, decryptCallback, dwnWorkers, cookieFile, referer, keyUrl, "videoDown/")
	if err != nil {
		return err
	}

    i := 0
    if err = h.SetResolution(i); err != nil {
        return err
    }

	downloaded = 0
	decrypted = 0
	segments = h.GetTotSegments()

	if err = h.ExtractVideo(); err != nil {
        return err
	}
    if isUseFFmpeg {
        if err = h.TempMerge(); err != nil {
            log.Error("%s", err)
            return err
        }
        if err = h.FFMerge(); err != nil {
		    log.Error("%s", err)
            return err
        }
    } else {
        if err = h.AppendMerge(); err != nil {
		    log.Error("%s", err)
            return err
        }
    }
    return nil
}

func processFFMerge() {
    wg := sync.WaitGroup{}
    wg.Add(syncWorkNum)
    var err error
    for i := 0; i < syncWorkNum; i ++ {
        go func() {
            for {
                val, ok := <-syncChan
                if ! ok {
                    break
                }
                if err = val.FFMerge(); err != nil {
                    log.Error("%s", err)
                } else {
                    msgChan <- fmt.Sprintf("%s merge finish!", val.FileAndPath)
                }
            }
            wg.Done()
        }()
    }
    wg.Wait()
    close(msgChan)
}

func init() {
    syncChan = make(chan * hlss.Hlss, 1000)
    msgChan  = make(chan string, 1000)
}
