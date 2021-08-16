package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"io"

	"github.com/schollz/progressbar/v3"

	"time"
)

func GetBaseUrl(url string) string {
	i := strings.LastIndex(url, "/")
	return url[0 : i+1]
}

func GetFileFromUrl(url string) string {
	i := strings.LastIndex(url, "/")
	return url[i+1:]
}

func AesDecrypt(key []byte, encrypted []byte, iv []byte) (decoded []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	if len(encrypted) < aes.BlockSize {
		err = errors.New("ciphertext block size is too short")
		return
	}

	stream := cipher.NewCBCDecrypter(block, iv)
	stream.CryptBlocks(encrypted, encrypted)

	decoded = encrypted
	return
}

func DecryptFileAppend(output *os.File, file string, key []byte, iv []byte) error {
	encrypted, err := ioutil.ReadFile(file)
	if err != nil {
		return errors.New("can't read the file on DecryptFileAppend : " + err.Error())
	}

	if decrypted, err := AesDecrypt(key, encrypted, iv); err != nil {
		return err
	} else {
		if _, err := output.Write(decrypted); err != nil {
			return err
		}
	}

	return nil
}

func FileAppend(output *os.File, file string) error {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return errors.New("can't read the file on FileAppend : " + err.Error())
	}

	if _, err := output.Write(content); err != nil {
		return err
	}

	return nil
}

func HttpRequest(method string, url string, cookies []*http.Cookie, referer string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	if len(cookies) > 0 {
		for _, c := range cookies {
			req.AddCookie(c)
		}
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http response status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return resp, nil
}

func DownloadShowBar(url string, filePath string) {
	var bar *progressbar.ProgressBar
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.104 Safari/537.36")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	f, _ := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	startT := time.Now().Unix()
	bar = progressbar.NewOptions(int(resp.ContentLength),
		// progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(70),
		progressbar.OptionSetDescription(filePath),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		// progressbar.OptionFullWidth(),
		// progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			endT := time.Now().Unix()
			fmt.Fprint(os.Stderr, fmt.Sprintf(" download spend time: %s\n", (time.Duration(endT-startT)*time.Second).String()))
		}),
	)
	io.Copy(io.MultiWriter(f, bar), resp.Body)
}
