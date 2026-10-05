package kconfig

// Option spellings retained as an offline probe inventory. Answers are generated
// from reference tools into llvm_capabilities; these lists contain no policy.

var linuxLLVMKconfigCCOptionsCommon = []string{}

var linuxLLVMKconfigCCOptionsX86 = []string{}

var linuxLLVMKconfigCCOptionsARM64 = []string{}

var linuxLLVMKconfigCCOptionsARMV7 = []string{}

var linuxLLVMKconfigCCOptionsRISCV64 = []string{}

var linuxLLVMKconfigCCOptionsPPC64LE = []string{}

var linuxLLVMKconfigLDOptions = []string{
	"--compress-debug-sections=zlib",
	"--compress-debug-sections=zstd",
	"--fix-cortex-a53-843419",
	"--gc-sections",
	"--orphan-handling=error",
	"--orphan-handling=warn",
}

var linuxLLVMKconfigLDOptionsRISCV64 = []string{
	"--no-relax-gp",
}

var linuxLLVMKbuildCommonOptions = []string{
	"cc_option\x00-Wformat-overflow",
	"cc_option\x00-Wformat-truncation",
	"cc_option\x00-Wmaybe-uninitialized",
	"cc_option\x00-Wno-address-of-packed-member",
	"cc_option\x00-Wno-fortify-source",
	"cc_option\x00-Wno-gnu",
	"cc_option\x00-Wno-missing-prototypes",
	"cc_option\x00-Wno-psabi",
	"cc_option\x00-Wno-stringop-overread",
	"cc_option\x00-Wno-stringop-truncation",
	"cc_option\x00-Wno-switch-unreachable",
	"cc_option\x00-Wno-tautological-constant-out-of-range-compare",
	"cc_option\x00-Wno-uninitialized",
	"cc_option\x00-Wno-unsequenced",
	"cc_option\x00-Wno-unused-but-set-variable",
	"cc_option\x00-Wno-unused-const-variable",
	"cc_option\x00-Wno-vla",
	"cc_option\x00-Wold-style-declaration",
	"cc_option\x00-Wout-of-line-declaration",
	"cc_option\x00-Wpacked-not-aligned",
	"cc_option\x00-Wrestrict",
	"cc_option\x00-Wstringop-overflow",
	"cc_option\x00-Wstringop-truncation",
	"cc_option\x00-Wunused-but-set-variable",
	"cc_option\x00-Wunused-const-variable",
	"cc_option\x00-Wvla-larger-than=1",
	"cc_option\x00-femit-struct-debug-detailed=any",
	"cc_option\x00-fno-addrsig",
	"cc_option\x00-fno-code-hoisting",
	"cc_option\x00-fno-conserve-stack",
	"cc_option\x00-fno-schedule-insns",
	"cc_option\x00-fmin-function-alignment=8",
	"cc_option\x00-fsanitize=kernel-memory",
	"cc_option\x00-fsched-pressure",
	"cc_option\x00-mabi=altivec",
	"cc_option\x00-mgeneral-regs-only",
	"cc_option\x00-mno-single-pic-base",
	"cc_option\x00-mrecord-mcount",
}

var linuxLLVMKbuildX86Options = []string{
	"cc_option\x00-Wa,-mtune=generic32",
	"cc_option\x00-Wa,-mrelax-relocations=no",
	"cc_option\x00-falign-jumps=0",
	"cc_option\x00-falign-jumps=1",
	"cc_option\x00-falign-loops=1",
	"cc_option\x00-fcf-protection=branch\x00-fno-jump-tables",
	"cc_option\x00-fcf-protection=none",
	"cc_option\x00-foptimize-sibling-calls",
	"cc_option\x00-maccumulate-outgoing-args",
	"cc_option\x00-mindirect-branch-cs-prefix",
	"cc_option\x00-mno-fp-ret-in-387",
	"cc_option\x00-mno-outline-atomics",
	"cc_option\x00-mpreferred-stack-boundary=4",
	"cc_option\x00-mskip-rax-setup",
	"cc_option\x00-mstack-alignment=16",
	"as_option\x00-Wa,-mtune=generic32",
	"ld_option\x00--no-ld-generated-unwind-info",
	"ld_option\x00--eh-frame-hdr",
	"ld_option\x00--no-warn-rwx-segments",
}

var linuxLLVMKbuildARM64Options = []string{
	"cc_option\x00-mabi=lp64",
	"cc_option\x00-mbranch-protection=none",
	"cc_option\x00-mno-outline-atomics",
	"as_option\x00-Wa,-march=armv8.2-a",
	"as_option\x00-Wa,-march=armv8.3-a",
	"as_option\x00-Wa,-march=armv8.4-a",
	"as_option\x00-Wa,-march=armv8.5-a",
	"ld_option\x00--no-apply-dynamic-relocs",
	"ld_option\x00-maarch64elf",
	"ld_option\x00-maarch64elfb",
}

var linuxLLVMKbuildX86ContextCandidates = []string{
	"-falign-loops=0",
	"-march=atom",
	"-march=c3",
	"-march=c3-2",
	"-march=core2",
	"-march=geode",
	"-march=k8",
	"-march=winchip-c6",
	"-march=winchip2",
	"-mtune=atom",
	"-mtune=core2",
	"-mtune=generic",
	"-mtune=i686",
	"-mtune=pentium2",
	"-mtune=pentium3",
	"-mtune=pentium4",
}
