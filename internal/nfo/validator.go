package nfo

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"
)

type scope struct {
	typ reflect.Type
}

type validator struct {
	funcs      map[string]bool
	tmpl       *template.Template
	scopeStack []scope
	errors     []error
	depth      int
}

// ValidateTemplate performs static analysis on the parsed template tree
// to ensure all field references and function calls are valid.
func ValidateTemplate(tmpl *template.Template) error {
	if tmpl == nil {
		return nil
	}

	v := &validator{
		funcs: buildFuncMap(),
		tmpl:  tmpl,
	}

	rootType := reflect.TypeFor[*Context]()
	v.scopeStack = []scope{{typ: rootType}}

	if tmpl.Tree != nil && tmpl.Root != nil {
		v.walkNode(tmpl.Root, tmpl.Name(), tmpl)
	}

	if len(v.errors) > 0 {
		// Just return the first error for simplicity
		return v.errors[0]
	}

	return nil
}

func buildFuncMap() map[string]bool {
	m := make(map[string]bool)
	// Built-in functions
	for _, name := range []string{"and", "call", "html", "index", "slice", "js", "len", "not", "or", "print", "printf", "println", "urlquery", "eq", "ge", "gt", "le", "lt", "ne"} {
		m[name] = true
	}
	// Our custom functions
	for name := range templateFuncs() {
		m[name] = true
	}

	return m
}

func (v *validator) walkNode(node parse.Node, tmplName string, t *template.Template) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parse.ListNode:
		v.walkListNode(n, tmplName, t)
	case *parse.ActionNode:
		v.walkNode(n.Pipe, tmplName, t)
	case *parse.IfNode:
		v.walkIfNode(n, tmplName, t)
	case *parse.RangeNode:
		v.walkRangeNode(n, tmplName, t)
	case *parse.WithNode:
		v.walkWithNode(n, tmplName, t)
	default:
		v.walkNodeFallback(node, tmplName, t)
	}
}

func (v *validator) walkNodeFallback(node parse.Node, tmplName string, t *template.Template) {
	switch n := node.(type) {
	case *parse.CommandNode:
		v.walkCommandNode(n, tmplName, t)
	case *parse.PipeNode:
		v.walkPipeNode(n, tmplName, t)
	case *parse.FieldNode:
		v.walkFieldNode(n, tmplName, t)
	case *parse.TemplateNode:
		v.walkTemplateNode(n, tmplName, t)
	}
}

func (v *validator) walkIfNode(n *parse.IfNode, tmplName string, t *template.Template) {
	v.walkNode(n.Pipe, tmplName, t)
	v.walkNode(n.List, tmplName, t)

	if n.ElseList != nil {
		v.walkNode(n.ElseList, tmplName, t)
	}
}

func (v *validator) walkListNode(n *parse.ListNode, tmplName string, t *template.Template) {
	for _, child := range n.Nodes {
		v.walkNode(child, tmplName, t)
	}
}

func (v *validator) walkPipeNode(n *parse.PipeNode, tmplName string, t *template.Template) {
	for _, cmd := range n.Cmds {
		v.walkNode(cmd, tmplName, t)
	}
}

func (v *validator) walkRangeNode(n *parse.RangeNode, tmplName string, t *template.Template) {
	v.walkNode(n.Pipe, tmplName, t)

	pipeType := v.inferPipelineType(n.Pipe)

	var elemType reflect.Type

	if pipeType != nil && (pipeType.Kind() == reflect.Slice || pipeType.Kind() == reflect.Array) {
		elemType = pipeType.Elem()
	}

	v.scopeStack = append(v.scopeStack, scope{typ: elemType})
	v.walkNode(n.List, tmplName, t)
	v.scopeStack = v.scopeStack[:len(v.scopeStack)-1]

	if n.ElseList != nil {
		v.walkNode(n.ElseList, tmplName, t)
	}
}

func (v *validator) walkWithNode(n *parse.WithNode, tmplName string, t *template.Template) {
	v.walkNode(n.Pipe, tmplName, t)

	pipeType := v.inferPipelineType(n.Pipe)
	v.scopeStack = append(v.scopeStack, scope{typ: pipeType})
	v.walkNode(n.List, tmplName, t)
	v.scopeStack = v.scopeStack[:len(v.scopeStack)-1]

	if n.ElseList != nil {
		v.walkNode(n.ElseList, tmplName, t)
	}
}

func (v *validator) walkCommandNode(n *parse.CommandNode, tmplName string, t *template.Template) {
	if len(n.Args) > 0 {
		v.checkCommandIdentifier(n, tmplName, t)
	}

	for _, arg := range n.Args {
		v.walkNode(arg, tmplName, t)
	}
}

func (v *validator) checkCommandIdentifier(n *parse.CommandNode, tmplName string, t *template.Template) {
	ident, ok := n.Args[0].(*parse.IdentifierNode)
	if !ok {
		return
	}

	if ident.Ident == "include" && len(n.Args) >= 2 {
		if strNode, ok := n.Args[1].(*parse.StringNode); ok {
			targetName := strNode.Text

			var passType reflect.Type

			if len(n.Args) >= 3 {
				passType = v.inferNodeType(n.Args[2])
			}

			v.walkTargetTemplate(targetName, passType)
		}
	} else if !v.funcs[ident.Ident] {
		v.errors = append(v.errors, fmt.Errorf("%w\ntemplate: %s:%d: executing \"%s\" at <%s>: unknown function \"%s\"", ErrTemplateSyntax, tmplName, getLineNumber(t, n), tmplName, ident.Ident, ident.Ident))
	}
}

func (v *validator) walkTemplateNode(n *parse.TemplateNode, tmplName string, t *template.Template) {
	var passType reflect.Type

	if n.Pipe != nil {
		passType = v.inferPipelineType(n.Pipe)
		v.walkNode(n.Pipe, tmplName, t)
	}

	v.walkTargetTemplate(n.Name, passType)
}

func (v *validator) walkTargetTemplate(targetName string, passType reflect.Type) {
	if v.depth > 20 {
		return
	}

	targetTmpl := v.tmpl.Lookup(targetName)

	if targetTmpl != nil && targetTmpl.Tree != nil && targetTmpl.Root != nil {
		v.scopeStack = append(v.scopeStack, scope{typ: passType})
		v.depth++
		v.walkNode(targetTmpl.Root, targetName, targetTmpl)
		v.depth--
		v.scopeStack = v.scopeStack[:len(v.scopeStack)-1]
	}
}

func (v *validator) walkFieldNode(n *parse.FieldNode, tmplName string, t *template.Template) {
	if typ := v.currentScope(); typ != nil {
		if !v.validateField(typ, n.Ident) {
			v.errors = append(v.errors, fmt.Errorf("%w\ntemplate: %s:%d: executing \"%s\" at <.%s>: can't evaluate field %s in type %s", ErrTemplateExecution, tmplName, getLineNumber(t, n), tmplName, strings.Join(n.Ident, "."), n.Ident[0], typ.String()))
		}
	}
}

func getLineNumber(t *template.Template, n parse.Node) int {
	if t != nil && t.Tree != nil {
		loc, _ := t.ErrorContext(n)

		parts := strings.Split(loc, ":")
		if len(parts) >= 2 {
			if line, err := strconv.Atoi(parts[1]); err == nil {
				return line
			}
		}
	}

	return 0
}

func (v *validator) currentScope() reflect.Type {
	if len(v.scopeStack) == 0 {
		return nil
	}

	return v.scopeStack[len(v.scopeStack)-1].typ
}

func (v *validator) lookupFieldOrMethod(typ reflect.Type, name string) (reflect.Type, bool) {
	if m, ok := typ.MethodByName(name); ok {
		if m.Type.NumOut() > 0 {
			return m.Type.Out(0), true
		}

		return nil, true
	}

	curr := typ
	if curr.Kind() == reflect.Pointer {
		curr = curr.Elem()
	}

	if curr.Kind() == reflect.Struct {
		if f, ok := curr.FieldByName(name); ok {
			return f.Type, true
		}
	}

	return nil, false
}

func (v *validator) validateField(typ reflect.Type, ident []string) bool {
	if typ == nil {
		return true // Cannot statically verify
	}

	curr := typ
	for _, name := range ident {
		nextType, ok := v.lookupFieldOrMethod(curr, name)
		if !ok {
			return false
		}

		curr = nextType
	}

	return true
}

func (v *validator) inferNodeType(node parse.Node) reflect.Type {
	switch n := node.(type) {
	case *parse.DotNode:
		return v.currentScope()
	case *parse.FieldNode:
		typ := v.currentScope()
		if typ == nil {
			return nil
		}

		curr := typ

		for _, name := range n.Ident {
			nextType, ok := v.lookupFieldOrMethod(curr, name)
			if !ok {
				return nil
			}

			curr = nextType
		}

		return curr
	case *parse.PipeNode:
		return v.inferPipelineType(n)
	}

	return nil
}

func (v *validator) inferPipelineType(pipe *parse.PipeNode) reflect.Type {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return nil
	}

	cmd := pipe.Cmds[0]
	if len(cmd.Args) == 0 {
		return nil
	}

	if field, ok := cmd.Args[0].(*parse.FieldNode); ok {
		typ := v.currentScope()
		if typ == nil {
			return nil
		}

		curr := typ
		for _, name := range field.Ident {
			if curr.Kind() == reflect.Pointer {
				curr = curr.Elem()
			}

			if curr.Kind() != reflect.Struct {
				return nil
			}

			f, ok := curr.FieldByName(name)
			if !ok {
				return nil
			}

			curr = f.Type
		}

		return curr
	}

	return nil
}
