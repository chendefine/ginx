package ginx

import "github.com/gin-gonic/gin/binding"

type HandleOption func(cfg *handleConfig)

var globalConfig = handleConfig{
	dataWrap: true,
	pbParse:  false,
	alwaysOK: false,

	invalidArgumentCode:     400,
	internalServerErrorCode: 500,
}

type handleConfig struct {
	dataWrap bool
	pbParse  bool
	alwaysOK bool

	invalidArgumentCode     int
	internalServerErrorCode int
}

func GlobalSetDataWrap(b bool) {
	globalConfig.dataWrap = b
}

func GlobalSetPbParse(b bool) {
	globalConfig.pbParse = b
}

func GlobalSetAlwaysOK(b bool) {
	globalConfig.alwaysOK = b
}

func GlobalSetInvalidArgumentCode(n int) {
	globalConfig.invalidArgumentCode = n
}

func GlobalSetInternalServerErrorCode(n int) {
	globalConfig.internalServerErrorCode = n
}

func EnableDataWrap() HandleOption {
	return func(cfg *handleConfig) {
		cfg.dataWrap = true
	}
}

func DisableDataWrap() HandleOption {
	return func(cfg *handleConfig) {
		cfg.dataWrap = false
	}
}

func EnablePbParse() HandleOption {
	return func(cfg *handleConfig) {
		cfg.pbParse = true
	}
}

func DisablePbParse() HandleOption {
	return func(cfg *handleConfig) {
		cfg.pbParse = false
	}
}

func EnableAlwaysOK() HandleOption {
	return func(cfg *handleConfig) {
		cfg.alwaysOK = true
	}
}

func DisableAlwaysOK() HandleOption {
	return func(cfg *handleConfig) {
		cfg.alwaysOK = true
	}
}

func SetInvalidArgumentCode(n int) HandleOption {
	return func(cfg *handleConfig) {
		cfg.invalidArgumentCode = n
	}
}

func SetInternalServerErrorCode(n int) HandleOption {
	return func(cfg *handleConfig) {
		cfg.internalServerErrorCode = n
	}
}

func SetJsonDecoderUseNumber(b bool) {
	binding.EnableDecoderUseNumber = b
}
