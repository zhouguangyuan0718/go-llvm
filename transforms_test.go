// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package llvm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func transformationModule(t *testing.T, source string) Module {
	t.Helper()
	ctx := NewContext()
	t.Cleanup(ctx.Dispose)
	path := filepath.Join(t.TempDir(), "transforms.ll")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	buf, err := NewMemoryBufferFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mod, err := ctx.ParseIR(buf)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mod.Dispose)
	return mod
}

func TestCloneFunctionAndLocalCleanup(t *testing.T) {
	mod := transformationModule(t, `
declare i32 @external(i32) "test.attribute"="kept"
@address = global i32 0
@alias = alias i32, i32* @address
define i32 @original(i32 %n, i1 %flag) noinline !dbg !3 {
entry:
  %not = xor i1 %flag, true, !dbg !4
  br i1 %not, label %fallback, label %fast, !dbg !4
fast:
  %value = call i32 @external(i32 %n), !dbg !4
  br label %merge
fallback:
  br label %merge
merge:
  %result = phi i32 [ %value, %fast ], [ 0, %fallback ]
  ret i32 %result, !dbg !4
}
!llvm.dbg.cu = !{!0}
!llvm.module.flags = !{!5}
!0 = distinct !DICompileUnit(language: DW_LANG_C, file: !1, producer: "test", isOptimized: false, runtimeVersion: 0, emissionKind: FullDebug)
!1 = !DIFile(filename: "source.go", directory: "/test")
!2 = !DISubroutineType(types: !6)
!3 = distinct !DISubprogram(name: "source.name", linkageName: "original", scope: !1, file: !1, line: 10, type: !2, scopeLine: 10, spFlags: DISPFlagDefinition, unit: !0)
!4 = !DILocation(line: 11, column: 1, scope: !3)
!5 = !{i32 2, !"Debug Info Version", i32 3}
!6 = !{}
`)
	original := mod.NamedFunction("original")
	before := original.String()
	clone := original.CloneFunction()
	clone.SetName("copy")
	if clone.Subprogram() == original.Subprogram() || clone.Subprogram().IsNil() {
		t.Fatal("clone must own distinct debug information")
	}
	clone.Subprogram().SetSubprogramLinkageName("copy")
	if !strings.Contains(mod.String(), `linkageName: "copy"`) || !strings.Contains(mod.String(), `linkageName: "original"`) {
		t.Fatal("debug linker names are not independent")
	}
	clone.Param(1).ReplaceAllUsesWith(ConstInt(mod.Context().Int1Type(), 1, false))
	for {
		changed := false
		for bb := clone.FirstBasicBlock(); !bb.IsNil(); bb = NextBasicBlock(bb) {
			changed = bb.SimplifyInstructionsInBlock() || changed
			changed = bb.ConstantFoldTerminator(true) || changed
		}
		changed = clone.RemoveUnreachableBlocks() || changed
		if !changed {
			break
		}
	}
	if original.String() != before || strings.Contains(clone.String(), "fallback") || strings.Contains(clone.String(), "phi ") {
		t.Fatalf("clone cleanup changed original or retained dead control flow:\n%s", clone.String())
	}
	if !strings.Contains(clone.String(), "@external(") || clone.GetEnumFunctionAttribute(AttributeKindID("noinline")).IsNil() {
		t.Fatal("external references or attributes were lost")
	}
	decl := mod.NamedFunction("external").CloneFunction()
	decl.SetName("external.copy")
	if !decl.IsDeclaration() || decl.GetStringAttributeAtIndex(-1, "test.attribute").GetStringValue() != "kept" {
		t.Fatal("declaration clone lost its attributes")
	}
	for _, name := range []string{"original", "copy", "external", "address", "alias"} {
		if mod.NamedValue(name).IsNil() {
			t.Errorf("missing named value %s", name)
		}
	}
	if !mod.NamedValue("absent").IsNil() {
		t.Fatal("unknown symbol found")
	}
	if err := VerifyModule(mod, ReturnStatusAction); err != nil {
		t.Fatal(err)
	}
}

func TestMustTailCallKinds(t *testing.T) {
	mod := transformationModule(t, `
declare i32 @callee(i32)
define i32 @forward(i32 %x) {
  %r = call i32 @callee(i32 %x)
  ret i32 %r
}
`)
	call := mod.NamedFunction("forward").FirstBasicBlock().FirstInstruction()
	for _, kind := range []TailCallKind{TailCallKindTail, TailCallKindNoTail, TailCallKindNone, TailCallKindMustTail} {
		call.SetTailCallKind(kind)
		if call.TailCallKind() != kind {
			t.Fatalf("tail kind did not round trip: %d", kind)
		}
		if err := VerifyModule(mod, ReturnStatusAction); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(call.String(), "musttail call") {
		t.Fatal("musttail was downgraded to an optional hint")
	}
}

func TestInlineAsmInfoAndMetadataConversion(t *testing.T) {
	ctx := NewContext()
	defer ctx.Dispose()
	typ := FunctionType(ctx.VoidType(), []Type{ctx.Int32Type()}, false)
	for _, sideEffects := range []bool{false, true} {
		asm := InlineAsm(typ, "# marker $0", "r", sideEffects, true, InlineAsmDialectIntel, true)
		info := asm.InlineAsmInfo()
		rebuilt := InlineAsm(info.Type, info.Assembly, info.Constraints, info.HasSideEffects, info.IsAlignStack, info.Dialect, info.CanThrow)
		if rebuilt != asm || info.HasSideEffects != sideEffects || !info.IsAlignStack || !info.CanThrow || info.Dialect != InlineAsmDialectIntel {
			t.Fatalf("inline asm properties changed: %+v", info)
		}
	}
	for _, md := range []Metadata{ctx.MDString("name"), ctx.MDNode(nil), ConstInt(ctx.Int32Type(), 7, false).ConstantAsMetadata()} {
		if ctx.MetadataAsValue(md).AsMetadata() != md {
			t.Fatal("metadata conversion changed identity")
		}
	}
}
