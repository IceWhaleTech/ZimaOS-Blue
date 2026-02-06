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

        // Link espeak-ng and its dependencies (from third_party)
        println!("cargo:rustc-link-search=native=../../third_party/espeak-ng/build/src/libespeak-ng");
        println!("cargo:rustc-link-search=native=../../third_party/espeak-ng/build/src/speechPlayer");
        println!("cargo:rustc-link-search=native=../../third_party/espeak-ng/build/src/ucd-tools");
        println!("cargo:rustc-link-search=native=../../third_party/espeak-ng/build");
        println!("cargo:rustc-link-lib=static=espeak-ng");
        println!("cargo:rustc-link-lib=static=speechPlayer");
        println!("cargo:rustc-link-lib=static=ucd");
        println!("cargo:rustc-link-lib=static=sonic");

        // Link whisper.cpp and ggml (from third_party)
        println!("cargo:rustc-link-search=native=../../third_party/whisper.cpp/build/src");
        println!("cargo:rustc-link-search=native=../../third_party/whisper.cpp/build/ggml/src");
        println!("cargo:rustc-link-search=native=../../third_party/whisper.cpp/build/ggml/src/ggml-metal");
        println!("cargo:rustc-link-search=native=../../third_party/whisper.cpp/build/ggml/src/ggml-blas");
        println!("cargo:rustc-link-lib=static=whisper");
        println!("cargo:rustc-link-lib=static=ggml");
        println!("cargo:rustc-link-lib=static=ggml-base");
        println!("cargo:rustc-link-lib=static=ggml-cpu");
        println!("cargo:rustc-link-lib=static=ggml-metal");
        println!("cargo:rustc-link-lib=static=ggml-blas");

        // Link libopus (from third_party)
        println!("cargo:rustc-link-search=native=../../third_party/opus-src/build");
        println!("cargo:rustc-link-lib=static=opus");

        // Link frameworks for whisper/ggml
        println!("cargo:rustc-link-lib=framework=Accelerate");
        println!("cargo:rustc-link-lib=framework=Metal");
        println!("cargo:rustc-link-lib=framework=MetalKit");
        println!("cargo:rustc-link-lib=framework=Foundation");

        // Link C++ standard library
        println!("cargo:rustc-link-lib=c++");

        // Rerun if the library changes
        println!("cargo:rerun-if-changed=lib/libecho.a");
    }

    tauri_build::build()
}
