package core

type Converter[From any, To any] interface {
	Convert() Codec[From, To]

	Parse() Codec[To, From]
}

type Codec[From any, To any] interface {
	Convert(source From) To
}
