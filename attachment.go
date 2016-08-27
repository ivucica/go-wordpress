package wordpress

import (
	"fmt"
	"github.com/wulijun/go-php-serialize/phpserialize"
)

const CacheKeyAttachment = "wp_attachment_%d"

// Attachment represents a WordPress attachment
type Attachment struct {
	Object

	FileName string `json:"file_name"`

	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`

	Caption string `json:"caption"`
	AltText string `json:"alt_text"`

	Url string `json:"url,omitempty"`
}

const FilterAfterGetAttachments = "after_get_attachments"
type FilterAfterGetAttachmentsFunc func(*WordPress, []*Attachment) ([]*Attachment, error)

// GetAttachments gets all attachment data from the database
func (wp *WordPress) GetAttachments(attachmentIds ...int64) ([]*Attachment, error) {
	var ret []*Attachment
	keyMap, _ := wp.cacheGetMulti(CacheKeyAttachment, attachmentIds, &ret)

	if len(keyMap) > 0 {
		missedIds := make([]int64, 0, len(keyMap))
		for _, indices := range keyMap {
			missedIds = append(missedIds, attachmentIds[indices[0]])
		}

		objects, err := wp.GetObjects(missedIds...)
		if err != nil {
			return nil, err
		}

		baseUrl, _ := wp.GetOption("upload_url_path")
		if baseUrl == "" {
			siteUrl, _ := wp.GetOption("siteurl")
			baseDir, _ := wp.GetOption("upload_path")
			if baseDir == "" {
				baseDir = "/wp-content/uploads"
			}

			baseUrl = siteUrl + baseDir
		}

		for _, obj := range objects {
			a := Attachment{Object: *obj}

			meta, err := a.GetMeta("_wp_attachment_metadata")
			if err != nil {
				return nil, err
			}

			if enc, ok := meta["_wp_attachment_metadata"]; ok && enc != "" {
				if dec, err := phpserialize.Decode(enc); err == nil {
					if meta, ok := dec.(map[interface{}]interface{}); ok {
						if file, ok := meta["file"].(string); ok {
							a.FileName = file
						}

						if width, ok := meta["width"].(int64); ok {
							a.Width = int(width)
						}

						if height, ok := meta["height"].(int64); ok {
							a.Height = int(height)
						}

						if imageMeta, ok := meta["image_meta"].(map[interface{}]interface{}); ok {
							if caption, ok := imageMeta["caption"].(string); ok {
								a.Caption = caption
							}

							if alt, ok := imageMeta["title"].(string); ok {
								a.AltText = alt
							}
						}
					}
				}
			}

			a.Url = baseUrl + a.Date.Format("/2006/01/") + a.FileName

			// insert into return set
			for _, index := range keyMap[fmt.Sprintf(CacheKeyAttachment, a.Id)] {
				ret[index] = &a
			}
		}

		for _, filter := range wp.filters[FilterAfterGetAttachments] {
			f, ok := filter.(FilterAfterGetAttachmentsFunc)
			if !ok {
				panic("got a bad filter for '" + FilterAfterGetAttachments + "'")
			}

			ret, err = f(wp, ret)
			if err != nil {
				return nil, err
			}
		}

		// just let this run, no callback is needed
		go wp.cacheSetByKeyMap(keyMap, ret)
	}

	var err MissingResourcesError
	for i, att := range ret {
		if att == nil {
			err = append(err, attachmentIds[i])
		} else {
			att.wp = wp
		}
	}

	if len(err) > 0 {
		return nil, err
	}

	return ret, nil
}

// QueryAttachments queries the database and returns all matching attachment ids
func (wp *WordPress) QueryAttachments(q *ObjectQueryOptions) ([]int64, error) {
	q.PostType = PostTypeAttachment

	return wp.QueryObjects(q)
}
