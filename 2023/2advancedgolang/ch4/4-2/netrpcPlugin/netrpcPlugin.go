package main

import (
	"io/ioutil"
	"os"
	"github.com/golang/protobuf/protoc-gen-go/generator"
	"github.com/golang/protobuf/proto"
)

type netrpcPlugin struct{ *generator.Generator }

func (p *netrpcPlugin) Name() string                { return "netrpc" }
func (p *netrpcPlugin) Init(g *generator.Generator) { p.Generator = g }
func (p *netrpcPlugin) GeneratorImports(file *generator.FileDescriptor) {
	if len(file.Service) > 0 {
		p.genImportCode(file)
	}
}
func (p *netrpcPlugin) Generator(file *generator.FileDescriptor) {
	for _, svc := range file.Service {
		p.genServiceCode(svc)
	}
}

func (p *netrpcPlugin) genImportCode(file *generator.FileDescriptor){
	p.P(`import "net/rpc"`)
}

func(p *netrpcPlugin) genServiceCode(svc *descriptor.ServiceDescriptorProto){
	p.P("// TODO: service code, Name = " + svc.GetName())
}

func init(){
	generator.RegisterPlugin(new (netrpcPlugin))
}

func main(){
	g:= generator.New()
	data,err:=ioutil.ReadAll(os.Stdin)
	if err!=nil{
		g.Error(err,"reading input")
	}
	if err:=proto.Unmarshal(data,g.Request);err!=nil{
		g.Error(err,"parsing input proto")
	}
	if len(g.Request.FileToGenerate)==0{
		g.Fail("no files to generate")
	}

	g.CommandLineParameters(g.Request.GetParameter())

	g.WrapTypes()
	g.SetPackageNames()
	g.BuildTypeNameMap()
	g.generateAllFiles()

	data,err=proto.Marshal(g.Response)
	if err!=nil{
		g.Error(err,"faied to write output proto")
	}
}
