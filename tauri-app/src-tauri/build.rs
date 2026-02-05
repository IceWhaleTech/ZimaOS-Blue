fn main() {
    // Platform-specific build configuration
    #[cfg(target_os = "macos")]
    {
        // Link the Go static library (libecho.a)
        println!("cargo:rustc-link-search=native=lib");
        println!("cargo:rustc-link-lib=static=echo");

        // Link required system frameworks for Go runtime
        println!("cargo:rustc-link-lib=framework=CoreFoundation");
        println!("cargo:rustc-link-lib=framework=Security");
        println!("cargo:rustc-link-lib=framework=SystemConfiguration");
        println!("cargo:rustc-link-lib=framework=IOKit");

        // Link Go runtime dependencies
        println!("cargo:rustc-link-lib=resolv");

        // Link espeak-ng for TTS support (from third_party)
        println!("cargo:rustc-link-search=native=../../third_party/espeak-ng/build/src/libespeak-ng");
        println!("cargo:rustc-link-lib=static=espeak-ng");

        // Link whisper.cpp for STT support (from third_party)
        println!("cargo:rustc-link-search=native=../../third_party/whisper.cpp/build/src");
        println!("cargo:rustc-link-lib=static=whisper");

        // Link Accelerate framework for whisper-cpp
        println!("cargo:rustc-link-lib=framework=Accelerate");

        // Link C++ standard library for whisper-cpp
        println!("cargo:rustc-link-lib=c++");

        // Rerun if the library changes
        println!("cargo:rerun-if-changed=lib/libecho.a");
    }

    tauri_build::build()
}
