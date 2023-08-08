package nested

// goverter:converter
type Converter interface {
	// goverter:map . Address
	// goverter:map . Address.StreetInfo
	// goverter:map Street Address.StreetInfo.Name
	Convert(FlatPerson) Person
}

type FlatPerson struct {
	Name    string
	Age     int
	Street  string
	ZipCode string
}

type Person struct {
	Name string
	Age  int
	Address
}
type Address struct {
	StreetInfo
	ZipCode string
}
type StreetInfo struct {
	Name string
}
