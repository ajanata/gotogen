package media

type Type string

const (
	TypeEyes  Type = "eyes"
	TypeMouth Type = "mouth"
	TypeNose  Type = "nose"
	TypeFull  Type = "full"
)

func (t Type) Size() (w int16, h int16) {
	switch t {
	case TypeEyes:
		return 16, 16
	case TypeNose:
		return 8, 8
	case TypeMouth:
		return 64, 16
	case TypeFull:
		return 64, 32
	default:
		return 0, 0
	}
}
