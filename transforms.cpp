// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// Generic LLVM utilities exposed to Go; policy belongs to the caller.
#include "transforms.h"
#include "llvm/IR/DebugInfoMetadata.h"
#include "llvm/IR/InlineAsm.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/Module.h"
#include "llvm/Transforms/Utils/Cloning.h"
#include "llvm/Transforms/Utils/Local.h"
using namespace llvm;

LLVMValueRef LLVMGoGetNamedValue(LLVMModuleRef M, const char *Name,
                                 size_t Len) {
  return wrap(unwrap(M)->getNamedValue(StringRef(Name, Len)));
}
LLVMValueRef LLVMGoCloneFunction(LLVMValueRef Fn) {
  ValueToValueMapTy Map;
  return wrap(CloneFunction(unwrap<Function>(Fn), Map));
}
LLVMBool LLVMGoSimplifyInstructionsInBlock(LLVMBasicBlockRef BB) {
  return SimplifyInstructionsInBlock(unwrap(BB));
}
LLVMBool LLVMGoConstantFoldTerminator(LLVMBasicBlockRef BB,
                                      LLVMBool DeleteDead) {
  return ConstantFoldTerminator(unwrap(BB), DeleteDead);
}
LLVMBool LLVMGoRemoveUnreachableBlocks(LLVMValueRef Fn) {
  return removeUnreachableBlocks(*unwrap<Function>(Fn));
}
void LLVMGoSetSubprogramLinkageName(LLVMMetadataRef SP, const char *Name,
                                    size_t Len) {
  auto *Subprogram = unwrap<DISubprogram>(SP);
  Subprogram->replaceLinkageName(
      MDString::get(Subprogram->getContext(), StringRef(Name, Len)));
}
void LLVMGoSetTailCallKind(LLVMValueRef Call, unsigned Kind) {
  unwrap<CallInst>(Call)->setTailCallKind(
      static_cast<CallInst::TailCallKind>(Kind));
}
unsigned LLVMGoGetTailCallKind(LLVMValueRef Call) {
  return unwrap<CallInst>(Call)->getTailCallKind();
}
LLVMGoInlineAsmInfo LLVMGoGetInlineAsmInfo(LLVMValueRef Asm) {
  auto *A = unwrap<InlineAsm>(Asm);
  const auto &Text = A->getAsmString();
  const auto &Constraints = A->getConstraintString();
  return {wrap(A->getFunctionType()),
          Text.data(),
          Constraints.data(),
          Text.size(),
          Constraints.size(),
          A->hasSideEffects(),
          A->isAlignStack(),
          A->canThrow(),
          static_cast<LLVMInlineAsmDialect>(A->getDialect())};
}
