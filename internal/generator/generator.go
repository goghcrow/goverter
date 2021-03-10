package generator

import (
	"fmt"
	"go/importer"
	"go/types"

	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/go-genconv/internal/comments"
)

type Config struct {
	Name string
}

func Generate(pattern string, mapping comments.Mapping, config Config) (*jen.File, error) {
	sources, err := importer.For("source", nil).Import(pattern)
	if err != nil {
		return nil, err
	}
	file := jen.NewFile(config.Name)
	file.HeaderComment("// Code generated by github.com/jmattheis/go-genconv, DO NOT EDIT.")

	for name, convConfig := range mapping {
		obj := sources.Scope().Lookup(name)
		if obj == nil {
			return nil, fmt.Errorf("%s: could not find %s", pattern, name)
		}

		// create the converter struct
		file.Type().Id(convConfig.Config.Name).Struct()

		gen := generator{name: convConfig.Config.Name, file: file, id: obj.Type().String()}

		// we checked in comments, that it is an interface
		interf := obj.Type().Underlying().(*types.Interface)
		for i := 0; i < interf.NumMethods(); i++ {
			method := interf.Method(i)
			if err := gen.addOuterMethod(method); err != nil {
				return nil, fmt.Errorf("%s: %s", method.FullName(), err)
			}

		}

	}
	return file, nil
}

type generator struct {
	name   string
	id     string
	file   *jen.File
	lookup map[string]map[string]string
}

func (g *generator) addOuterMethod(method *types.Func) error {
	signature, ok := method.Type().(*types.Signature)
	if !ok {
		return fmt.Errorf("expected signature %#v", method.Type())
	}
	params := signature.Params()
	if params.Len() != 1 {
		return fmt.Errorf("expected signature to have only one parameter")
	}
	result := signature.Results()
	if result.Len() != 1 {
		return fmt.Errorf("expected signature to have only one parameter")
	}
	source := params.At(0).Type()
	target := result.At(0).Type()

	return g.addMethod(method.Name(), source, target)
}

func (g *generator) addMethod(name string, source, target types.Type) error {
	jenSource, err := toCode(source, jen.Id("source"))
	if err != nil {
		return fmt.Errorf("source: %s", err)
	}
	jenTarget, err := toCode(target, empty())
	if err != nil {
		return fmt.Errorf("result: %s", err)
	}
	block, err := g.BuildBlock(jen.Return(), jen.Id("source"), source, target, 0)
	if err != nil {
		return fmt.Errorf("body: %s", err)
	}
	g.file.Func().Params(jen.Id("c").Op("*").Id(g.name)).Id(name).
		Params(jenSource).Params(jenTarget).
		Block(block...)
	return nil
}

func (g *generator) BuildBlock(end *jen.Statement, input *jen.Statement, source types.Type, target types.Type, level uint8) ([]jen.Code, error) {
	stmt := []jen.Code{}
	switch cast := target.(type) {
	case *types.Named:
		if utype, ok := cast.Underlying().(*types.Struct); ok {
			jenutype, err := toCode(target, jen.Var().Id("target"))
			if err != nil {
				return nil, fmt.Errorf("%s: %s", target.String(), err)
			}

			usource, ok := source.Underlying().(*types.Struct)
			if !ok {
				return nil, fmt.Errorf("cannot convert %s to %s", source.String(), target.String())
			}

			sourceMethods := map[string]*types.Var{}
			for i := 0; i < usource.NumFields(); i++ {
				m := usource.Field(i)
				sourceMethods[m.Name()] = m
			}

			stmt = append(stmt, jenutype)

			for i := 0; i < utype.NumFields(); i++ {
				targetField := utype.Field(i)
				sourceField, ok := sourceMethods[targetField.Name()]
				if !ok {
					return nil, fmt.Errorf("source %s does not have a field named %s", source.String(), targetField.Name())
				}

				inner, err := g.BuildBlock(jen.Id("target").Dot(targetField.Name()).Op("="), input.Clone().Dot(sourceField.Name()), sourceField.Type(), targetField.Type(), level+1)
				if err != nil {
					return nil, fmt.Errorf("%s.%s: %s", cast.String(), targetField.Name(), err)
				}

				stmt = append(stmt, inner...)
			}

			stmt = append(stmt, end.Clone().Id("target"))

		} else {
			return stmt, fmt.Errorf("only named struct types are currently supported was %s", cast.Underlying().String())
		}
	case *types.Basic:
		basicSource, ok := source.(*types.Basic)
		if !ok {
			return nil, fmt.Errorf("cannot convert %s to %s", source.String(), target.String())
		}
		if basicSource.Kind() == cast.Kind() {
			stmt = append(stmt, end.Add(input))
		} else {
			return nil, fmt.Errorf("cannot convert %s to %s", source.String(), target.String())
		}
	default:
		return nil, fmt.Errorf("cannot handle: %#v", target)
	}
	return stmt, nil
}

func toCode(t types.Type, st *jen.Statement) (jen.Code, error) {
	switch cast := t.(type) {
	case *types.Named:
		return st.Qual(cast.Obj().Pkg().Path(), cast.Obj().Name()), nil
	case *types.Map:
		key, err := toCode(cast.Key(), empty())
		if err != nil {
			return key, err
		}
		return toCode(cast.Elem(), st.Map(key))
	case *types.Slice:
		return toCode(cast.Elem(), st.Index())
	case *types.Array:
		return toCode(cast.Elem(), st.Index(jen.Lit(cast.Len)))
	case *types.Pointer:
		return toCode(cast.Elem(), st.Op("*"))
	case *types.Basic:
		switch cast.Kind() {
		case types.String:
			return st.String(), nil
		case types.Int:
			return st.Int(), nil
		case types.Int8:
			return st.Int8(), nil
		case types.Int16:
			return st.Int16(), nil
		case types.Int32:
			return st.Int32(), nil
		case types.Int64:
			return st.Int64(), nil
		case types.Uint:
			return st.Uint(), nil
		case types.Uint8:
			return st.Uint8(), nil
		case types.Uint16:
			return st.Uint16(), nil
		case types.Uint32:
			return st.Uint32(), nil
		case types.Uint64:
			return st.Uint64(), nil
		case types.Bool:
			return st.Bool(), nil
		case types.Complex128:
			return st.Complex128(), nil
		case types.Complex64:
			return st.Complex64(), nil
		case types.Float32:
			return st.Float32(), nil
		case types.Float64:
			return st.Float64(), nil
		}
	}
	return nil, fmt.Errorf("unsupported type " + t.String())
}
func empty() *jen.Statement {
	return &jen.Statement{}

}
