package zotero

import (
	"encoding/json"
	"fmt"
	"github.com/goph/emperror"
	"strconv"
)

type TagMeta struct {
	Type     int64 `json:"type"`
	NumItems int64 `json:"numItems"`
}

type Tag struct {
	Tag   string      `json:"tag"`
	Links interface{} `json:"links,omitempty"`
	Meta  *TagMeta    `json:"meta,omitempty"`
}

func (group *Group) CreateTagDB(tag Tag) error {
	group.zot.logger.Debugf("Creating Tag %s", tag.Tag)
	metastr, err := json.Marshal(tag.Meta)
	if err != nil {
		return emperror.Wrapf(err, "cannot marshal meta %v", tag.Meta)
	}
	sqlstr := fmt.Sprintf("INSERT INTO %s.tags (tag, meta, library) VALUES( $1, $2, $3)", group.zot.dbSchema)
	params := []interface{}{
		tag.Tag,
		metastr,
		group.Id,
	}
	_, err = group.zot.db.Exec(sqlstr, params...)
	if err != nil {
		if IsUniqueViolation(err, "pk_tags") {
			return nil
		}
		return emperror.Wrapf(err, "cannot execute %s: %v", sqlstr, params)
	}
	return nil
}

func (group *Group) DeleteTagDB(tag string) error {
	group.zot.logger.Infof("deleting Tag %s", tag)
	sqlstr := fmt.Sprintf("DELETE FROM %s.tags WHERE tag=$1 and library=$2", group.zot.dbSchema)

	params := []interface{}{
		tag,
		group.Id,
	}
	if _, err := group.zot.db.Exec(sqlstr, params...); err != nil {
		return emperror.Wrapf(err, "error executing %s: %v", sqlstr, params)
	}
	return nil
}

func (group *Group) GetTagsVersion(sinceVersion int64) (*[]Tag, error) {
	var endpoint string
	endpoint = fmt.Sprintf("/groups/%v/tags", group.Id)
	group.zot.logger.Infof("rest call: %s", endpoint)

	resp, err := group.zot.client.R().
		SetHeader("Accept", "application/json").
		SetQueryParam("since", strconv.FormatInt(sinceVersion, 10)).
		Get(endpoint)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot get current key from %s", endpoint)
	}
	rawBody := resp.Body()
	tags := &[]Tag{}
	if err := json.Unmarshal(rawBody, tags); err != nil {
		return nil, emperror.Wrapf(err, "cannot unmarshal %s", string(rawBody))
	}
	return tags, nil
}

func (group *Group) SyncTags() (int64, error) {
	if !group.CanDownload() {
		return 0, nil
	}
	group.zot.logger.Infof("Syncing tags of group #%v", group.Id)
	var counter int64
	tagList, err := group.GetTagsVersion(group.Version)
	if err != nil {
		return counter, emperror.Wrapf(err, "cannot get tag versions")
	}
	for _, tag := range *tagList {
		if err := group.CreateTagDB(tag); err != nil {
			return 0, emperror.Wrapf(err, "cannot create tag %v", tag.Tag)
		}
	}

	group.zot.logger.Infof("Syncing tags of group #%v done. %v tags changed", group.Id, counter)
	return counter, nil
}
