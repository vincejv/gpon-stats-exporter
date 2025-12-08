package device

import "github.com/vincejv/gpon-parser/model"

type OntDevice interface {
	GetOpticalInfo() *model.OpticalStats
	GetDeviceInfo() *model.DeviceStats
	GetGponUrl() string
	UpdateCachedPage()
}

type HG6245D_Globe struct {
}

type AN5506_Stock struct {
}

type ZTEF670L struct {
}

type ZLTG3000A struct {
}

type ZLTG202 struct {
}

type SKYW_GN struct {
}

type NOKIA_G010S struct {
}
