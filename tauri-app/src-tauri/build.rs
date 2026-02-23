fn main() {
    // Platform-specific build configuration
    #[cfg(target_os = "windows")]
    {
        // Link Go static library (libblue.a)
        let lib_path = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("lib");
        println!("cargo:rustc-link-search=native={}", lib_path.display());
        println!("cargo:rustc-link-lib=static=blue");

        // Windows system libraries for Go runtime
        println!("cargo:rustc-link-lib=ws2_32");
        println!("cargo:rustc-link-lib=userenv");
        println!("cargo:rustc-link-lib=ntdll");
        println!("cargo:rustc-link-lib=winmm");

        // Link C++ standard library for Windows TTS/ASR (speech_tts_windows.cpp in libblue.a)
        // Note: Using dynamic linking due to C runtime conflicts with static linking
        // The DLL will be automatically packaged by build-all.ps1
        println!("cargo:rustc-link-lib=stdc++");

        // Note: espeak-ng, kokoro, and related libraries are no longer compiled on Windows
        // Windows uses native TTS/ASR APIs instead

        // Force rebuild when libblue.a changes
        println!("cargo:rerun-if-changed=lib/libblue.a");
        // Also watch the lib directory
        println!("cargo:rerun-if-changed=lib");
    }

    #[cfg(target_os = "macos")]
    {
        // Link the Go static library (libblue.a)
        println!("cargo:rustc-link-search=native=lib");
        println!("cargo:rustc-link-lib=static=blue");

        // Link required system frameworks for Go runtime
        println!("cargo:rustc-link-lib=framework=CoreFoundation");
        println!("cargo:rustc-link-lib=framework=Security");
        println!("cargo:rustc-link-lib=framework=SystemConfiguration");
        println!("cargo:rustc-link-lib=framework=IOKit");

        // Link Go runtime dependencies
        println!("cargo:rustc-link-lib=resolv");

        // Foundation is always needed (Go runtime uses it on macOS)
        println!("cargo:rustc-link-lib=framework=Foundation");

        // Rerun if the library changes
        println!("cargo:rerun-if-changed=lib/libblue.a");
    }

    tauri_build::build()
}
