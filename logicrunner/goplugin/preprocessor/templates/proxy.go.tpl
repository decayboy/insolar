package {{ .PackageName }}

import (
	{{- range $import, $i := .Imports }}
		{{$import}}
	{{- end }}
)

{{ range $typeStruct := .Types }}
	{{- $typeStruct }}
{{ end }}

// ClassReference to class of this contract
var ClassReference = core.NewRefFromBase58("{{ .ClassReference }}")

// {{ .ContractType }} holds proxy type
type {{ .ContractType }} struct {
	Reference core.RecordRef
}

// ContractConstructorHolder holds logic with object construction
type ContractConstructorHolder struct {
	constructorName string
	argsSerialized []byte
}

// AsChild saves object as child
func (r *ContractConstructorHolder) AsChild(objRef core.RecordRef) (*{{ .ContractType }}, error) {
	ref, err := proxyctx.Current.SaveAsChild(objRef, ClassReference, r.constructorName, r.argsSerialized)
	if err != nil {
		return nil, err
	}
	return &{{ .ContractType }}{Reference: ref}, nil
}

// AsDelegate saves object as delegate
func (r *ContractConstructorHolder) AsDelegate(objRef core.RecordRef) (*{{ .ContractType }}, error) {
	ref, err := proxyctx.Current.SaveAsDelegate(objRef, ClassReference, r.constructorName, r.argsSerialized)
	if err != nil {
		return nil, err
	}
	return &{{ .ContractType }}{Reference: ref}, nil
}

// GetObject returns proxy object
func GetObject(ref core.RecordRef) (r *{{ .ContractType }}) {
	return &{{ .ContractType }}{Reference: ref}
}

// GetClass returns reference to the class
func GetClass() core.RecordRef {
	return ClassReference
}

// GetImplementationFrom returns proxy to delegate of given type
func GetImplementationFrom(object core.RecordRef) (*{{ .ContractType }}, error) {
	ref, err := proxyctx.Current.GetDelegate(object, ClassReference)
	if err != nil {
		return nil, err
	}
	return GetObject(ref), nil
}

{{ range $func := .ConstructorsProxies }}
// {{ $func.Name }} is constructor
func {{ $func.Name }}( {{ $func.Arguments }} ) *ContractConstructorHolder {
	{{ $func.InitArgs }}

	var argsSerialized []byte
	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		panic(err)
	}

	return &ContractConstructorHolder{constructorName: "{{ $func.Name }}", argsSerialized: argsSerialized}
}
{{ end }}

// GetReference returns reference of the object
func (r *{{ $.ContractType }}) GetReference() core.RecordRef {
	return r.Reference
}

// GetClass returns reference to the class
func (r *{{ $.ContractType }}) GetClass() core.RecordRef {
	return ClassReference
}

{{ range $method := .MethodsProxies }}
// {{ $method.Name }} is proxy generated method
func (r *{{ $.ContractType }}) {{ $method.Name }}( {{ $method.Arguments }} ) ( {{ $method.ResultsTypes }} ) {
	{{ $method.InitArgs }}
	var argsSerialized []byte

	{{ $method.ResultZeroList }}

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return {{ $method.ResultsWithErr }}
	}

	res, err := proxyctx.Current.RouteCall(r.Reference, true, "{{ $method.Name }}", argsSerialized)
	if err != nil {
		return {{ $method.ResultsWithErr }}
	}

	err = proxyctx.Current.Deserialize(res, &ret)
	if err != nil {
		return {{ $method.ResultsWithErr }}
	}

	if {{ $method.ErrorVar }} != nil {
		return {{ $method.Results }}
	}
	return {{ $method.ResultsNilError }}
}

// {{ $method.Name }}NoWait is proxy generated method
func (r *{{ $.ContractType }}) {{ $method.Name }}NoWait( {{ $method.Arguments }} ) error {
	{{ $method.InitArgs }}
	var argsSerialized []byte

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	_, err = proxyctx.Current.RouteCall(r.Reference, false, "{{ $method.Name }}", argsSerialized)
	if err != nil {
		return err
	}

	return nil
}
{{ end }}
