package office

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// ConverterRequest is the ONLYOFFICE POST /converter JSON body.
type ConverterRequest struct {
	Async      bool               `json:"async"`
	FileType   string             `json:"filetype"`
	Key        string             `json:"key"`
	OutputType string             `json:"outputtype"`
	Title      string             `json:"title"`
	URL        string             `json:"url"`
	Password   string             `json:"password"`
	Token      string             `json:"token"`
	Thumbnail  *ConverterThumbnail `json:"thumbnail"`
}

// ConverterThumbnail matches ONLYOFFICE thumbnail object.
type ConverterThumbnail struct {
	Width  int  `json:"width"`
	Height int  `json:"height"`
	Aspect int  `json:"aspect"`
	First  bool `json:"first"`
}

// ConverterResponse is returned when Accept includes application/json.
type ConverterResponse struct {
	EndConvert bool   `json:"endConvert"`
	FileType   string `json:"fileType"`
	FileURL    string `json:"fileUrl"`
	Percent    int    `json:"percent"`
	Error      any    `json:"error,omitempty"`
}

var convKeySafe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// ParseConverterRequest decodes JSON accepting ONLYOFFICE and FileBrowser field casing.
func ParseConverterRequest(raw []byte) (ConverterRequest, error) {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return ConverterRequest{}, err
	}
	getString := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := generic[k]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil && s != "" {
					return s
				}
			}
		}
		return ""
	}
	req := ConverterRequest{
		FileType:   getString("filetype", "Filetype", "fileType"),
		Key:        getString("key", "Key"),
		OutputType: getString("outputtype", "outputType", "OutputType"),
		Title:      getString("title", "Title"),
		URL:        getString("url", "URL"),
		Password:   getString("password", "Password"),
		Token:      getString("token", "Token"),
	}
	if v, ok := generic["async"]; ok {
		_ = json.Unmarshal(v, &req.Async)
	}
	for _, k := range []string{"thumbnail", "Thumbnail"} {
		if v, ok := generic[k]; ok {
			var thumb ConverterThumbnail
			if json.Unmarshal(v, &thumb) == nil {
				req.Thumbnail = &thumb
				break
			}
		}
	}
	if req.Key == "" {
		return ConverterRequest{}, fmt.Errorf("converter: missing key")
	}
	if req.URL == "" {
		return ConverterRequest{}, fmt.Errorf("converter: missing url")
	}
	if req.FileType == "" {
		return ConverterRequest{}, fmt.Errorf("converter: missing filetype")
	}
	if req.OutputType == "" {
		return ConverterRequest{}, fmt.Errorf("converter: missing outputtype")
	}
	req.FileType = strings.TrimPrefix(strings.ToLower(req.FileType), ".")
	req.OutputType = strings.TrimPrefix(strings.ToLower(req.OutputType), ".")
	return req, nil
}

// ConvCacheDirName returns the converter cache subdirectory for key+outputtype.
func ConvCacheDirName(key, outputType string) string {
	key = convKeySafe.ReplaceAllString(key, "_")
	if key == "" {
		key = "unknown"
	}
	outputType = strings.TrimPrefix(strings.ToLower(outputType), ".")
	return "conv_" + key + "_" + outputType
}

// ConvOutputBasename picks the on-disk output filename inside the conv cache dir.
func ConvOutputBasename(outputType string, thumb *ConverterThumbnail) string {
	outputType = strings.TrimPrefix(strings.ToLower(outputType), ".")
	if thumb != nil && !thumb.First && outputType != "zip" {
		return "output.zip"
	}
	switch outputType {
	case "jpeg":
		return "output.jpg"
	default:
		return "output." + outputType
	}
}

// ConvOutputFileType is the fileType field in the converter response.
func ConvOutputFileType(outputType string, thumb *ConverterThumbnail) string {
	outputType = strings.TrimPrefix(strings.ToLower(outputType), ".")
	if thumb != nil && !thumb.First && outputType != "zip" {
		return "zip"
	}
	if outputType == "jpeg" {
		return "jpg"
	}
	return outputType
}

// ConvFileURL builds a public URL for a converted artifact.
func ConvFileURL(publicOrigin, basePath, cacheDirName, outputName, title string) string {
	publicOrigin = strings.TrimSuffix(publicOrigin, "/")
	basePath = strings.TrimSuffix(basePath, "/")
	if basePath == "/" {
		basePath = ""
	}
	u := publicOrigin
	if basePath != "" {
		u += basePath
	}
	u += "/cache/files/" + cacheDirName + "/" + outputName
	q := url.Values{}
	if title != "" {
		q.Set("filename", path.Base(title))
	}
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}
	return u
}
