package resource

import (
	"context"
	"errors"
	"fmt"
	"github.com/qauzy/mat/common/utils"
	"github.com/qauzy/mat/tunnel/statistic"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	matHttp "github.com/qauzy/mat/component/http"
	types "github.com/qauzy/mat/constant/provider"
)

type FileVehicle struct {
	path string
}

func (f *FileVehicle) Type() types.VehicleType {
	return types.File
}

func (f *FileVehicle) Path() string {
	return f.path
}

func (f *FileVehicle) Read() ([]byte, error) {
	return os.ReadFile(f.path)
}

func (f *FileVehicle) Proxy() string {
	return ""
}

func NewFileVehicle(path string) *FileVehicle {
	return &FileVehicle{path: path}
}

type HTTPVehicle struct {
	url    string
	path   string
	proxy  string
	header http.Header
}

func (h *HTTPVehicle) Url() string {
	return h.url
}

func (h *HTTPVehicle) Type() types.VehicleType {
	return types.HTTP
}

func (h *HTTPVehicle) Path() string {
	return h.path
}

func (h *HTTPVehicle) Proxy() string {
	return h.proxy
}

func (h *HTTPVehicle) Read() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	up, down := statistic.DefaultManager.Statistic()
	resp, err := matHttp.HttpRequestWithProxy(ctx, fmt.Sprintf("%s&bit=%d", h.url, up+down), http.MethodGet, h.header, nil, h.proxy)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errors.New(resp.Status)
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(resp.Header.Get("X-UUID"))

	//是加密数据
	if key != "" {
		buf, err = utils.Decrypt([]byte(key[6:]), buf)
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func NewHTTPVehicle(url string, path string, proxy string, header http.Header) *HTTPVehicle {
	return &HTTPVehicle{url, path, proxy, header}
}
