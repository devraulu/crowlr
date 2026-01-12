package process

import "github.com/PuerkitoBio/purell"

func Normalize(url string) (string, error) {
	flags := purell.FlagLowercaseScheme |
		purell.FlagLowercaseHost |
		purell.FlagRemoveDefaultPort |
		purell.FlagRemoveFragment |
		purell.FlagDecodeUnnecessaryEscapes |
		purell.FlagSortQuery |
		purell.FlagRemoveDuplicateSlashes |
		purell.FlagRemoveDotSegments

	return purell.NormalizeURLString(url, flags)
}
