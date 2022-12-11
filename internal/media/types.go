package media

type Type string

const (
	TypeEye   Type = "eye"
	TypeMouth Type = "mouth"
	TypeNose  Type = "nose"
	TypeFull  Type = "full"
)

func (t Type) Size() (w int16, h int16) {
	switch t {
	case TypeEye:
		return 24, 12
	case TypeNose:
		return 12, 12
	case TypeMouth:
		return 48, 18
	case TypeFull:
		return 64, 32
	default:
		return 0, 0
	}
}
