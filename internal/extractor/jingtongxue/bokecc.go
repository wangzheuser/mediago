package jingtongxue

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor/shared"
	"github.com/Sophomoresty/mediago/internal/util"
)

// Jingtongxue's BokeCC endpoint has appeared as both JSONP/JSON (copies[]) and
// XML (<copy>...) in the restored sources.  Prefer the source-aligned JSONP
// parser and keep the shared XML resolver as a compatibility fallback.
func resolveJingtongxueBokeCC(c *util.Client, vid, siteid string, headers map[string]string, quality string) (string, error) {
	if vid == "" || siteid == "" {
		return "", fmt.Errorf("bokecc: missing vid or siteid")
	}
	apiURL := fmt.Sprintf(urlBokeCCVideoAPI, url.QueryEscape(vid), url.QueryEscape(siteid))
	body, err := c.GetString(apiURL, headers)
	if err != nil {
		return "", fmt.Errorf("bokecc fetch: %w", err)
	}
	if u := pickBokeCCCopy(parseBokeCCPayload(body), quality); u != "" {
		return u, nil
	}
	if u := pickBokeCCXMLCopy([]byte(body), quality); u != "" {
		return u, nil
	}
	return shared.BokeCCResolve(c, vid, siteid, headers)
}

func parseBokeCCPayload(text string) []map[string]any {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if strings.HasSuffix(text, ")") && strings.Contains(text, "(") {
		text = text[strings.Index(text, "(")+1 : len(text)-1]
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(text), &root); err != nil {
		return nil
	}
	return jtxMapRecords(root["copies"])
}

func pickBokeCCCopy(copies []map[string]any, quality string) string {
	type candidate struct {
		quality int64
		url     string
	}
	var out []candidate
	for _, copy := range copies {
		mediaType := int64FromAny(copy["mediatype"])
		if mediaType != 0 && mediaType != 1 {
			continue
		}
		playURL := normalizeURL(firstText(copy["playurl"], copy["backupurl"]), urlReferer)
		if playURL == "" {
			continue
		}
		out = append(out, candidate{quality: int64FromAny(copy["quality"]), url: playURL})
	}
	if len(out) == 0 {
		return ""
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].quality > out[j].quality })
	index := bokeCCQualityIndex(quality)
	if index >= len(out) {
		index = len(out) - 1
	}
	return out[index].url
}

func pickBokeCCXMLCopy(body []byte, quality string) string {
	var root struct {
		Copies []struct {
			Quality   int64  `xml:"quality"`
			PlayURL   string `xml:"playurl"`
			BackupURL string `xml:"backupurl"`
			MediaType int64  `xml:"mediatype"`
		} `xml:"copy"`
	}
	if err := xml.Unmarshal(body, &root); err != nil {
		return ""
	}
	copies := make([]map[string]any, 0, len(root.Copies))
	for _, copy := range root.Copies {
		copies = append(copies, map[string]any{
			"quality":   copy.Quality,
			"playurl":   copy.PlayURL,
			"backupurl": copy.BackupURL,
			"mediatype": copy.MediaType,
		})
	}
	return pickBokeCCCopy(copies, quality)
}

func bokeCCQualityIndex(quality string) int {
	switch strings.ToLower(strings.TrimSpace(quality)) {
	case "sd", "ld", "low", "360p", "360", "480p", "480":
		return 2
	case "hd", "720p", "720", "medium", "normal":
		return 1
	default:
		return 0
	}
}

func jtxMapRecords(v any) []map[string]any {
	switch x := v.(type) {
	case []map[string]any:
		return x
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, item := range x {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

var jtxBokeCCSeeds = mustDecodeBokeCCSeeds(
	"Uglq1TA2pTi/QKOegfPX+3zjOYKbL/+HNI5DRMTe6ctUe5QypsIjPe5MlQtC+sNOCC6hZijZJLJ2W6JJbYvRJXL49mSGaJgW1KRczF1ltpJscEhQ/e252l4VRlenjZ2EkNirAIy80wr35FgFuLNFBtAsHo/KPw8Cwa+9AwETims6kRFBT2fc6pfyz87wtOZzlqx0IuetNYXi+TfoHHXfbkfxGnEdKcWJb7diDqoYvhv8Vj5LxtJ5IJrbwP54zVr0H92oM4gHxzGxEhBZJ4DsX2BRf6kZtUoNLeV6n5PJnO+g4DtNrir1sMjruzyDU5lhFysEfrp31ibhaRRjVSEMfQ==",
	"Y1UhDH1SCWrVMDalOL9Ao56B89f7fOM5gpsv/4c0jkNExN7py1R7lDKmwiM97kyVC0L6w04ILqFmKNkksnZboklti9Elcvj2ZIZomBbUpFzMXWW2kmxwSFD97bnaXhVGV6eNnYSQ2KsAjLzTCvfkWAW4s0UG0Cwej8o/DwLBr70DAROKazqREUFPZ9zql/LPzvC05nOWrHQi5601heL5N+gcdd9uR/EacR0pxYlvt2IOqhi+G/xWPkvG0nkgmtvA/njNWvQf3agziAfHMbESEFkngOxfYFF/qRm1Sg0t5Xqfk8mc76DgO02uKvWwyOu7PINTmWEXKwR+unfWJuFpFA==",
	"c5asdCLnrTWF4vk36Bx1325H8RpxHSnFiW+3Yg6qGL4b/FY+S8bSeSCa28D+eM1a9B/dqDOIB8cxsRIQWSeA7F9gUX+pGbVKDS3lep+TyZzvoOA7Ta4q9bDI67s8g1OZYRcrBH66d9Ym4WkUY1UhDH1SCWrVMDalOL9Ao56B89f7fOM5gpsv/4c0jkNExN7py1R7lDKmwiM97kyVC0L6w04ILqFmKNkksnZboklti9Elcvj2ZIZomBbUpFzMXWW2kmxwSFD97bnaXhVGV6eNnYSQ2KsAjLzTCvfkWAW4s0UG0Cwej8o/DwLBr70DAROKazqREUFPZ9zql/LPzvC05g==",
)

func mustDecodeBokeCCSeeds(values ...string) [][]byte {
	out := make([][]byte, 0, len(values))
	for _, value := range values {
		b, _ := base64.StdEncoding.DecodeString(value)
		out = append(out, b)
	}
	return out
}

// decryptJingtongxueBokeCCKey mirrors Python _decrypt_bokecc_key.  It is kept
// package-private for future HLS key rewriting paths.
func decryptJingtongxueBokeCCKey(encKey []byte, vid string) []byte {
	if len(encKey) == 0 || vid == "" {
		return nil
	}
	seedIndex := int(encKey[0])
	if seedIndex < 0 || seedIndex >= len(jtxBokeCCSeeds) {
		return nil
	}
	vidBytes := []byte(vid)
	if len(vidBytes) == 0 {
		return nil
	}
	src := encKey[1:]
	out := make([]byte, 0, 16)
	for i := 0; i < len(src) && i < 20; i++ {
		idx := int(src[i] ^ vidBytes[i%len(vidBytes)])
		if idx < 0 || idx >= len(jtxBokeCCSeeds[seedIndex]) {
			continue
		}
		out = append(out, jtxBokeCCSeeds[seedIndex][idx])
		if len(out) == 16 {
			return out
		}
	}
	if len(out) == 16 {
		return out
	}
	return nil
}
