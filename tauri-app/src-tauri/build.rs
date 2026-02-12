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

        // Build libsonic.a from espeak-ng's compiled object if it doesn't exist
        let espeak_build = std::path::Path::new("../../third_party/espeak-ng/build");
        let sonic_lib = espeak_build.join("libsonic.a");
        let sonic_obj = espeak_build.join("CMakeFiles/sonic.dir/_deps/sonic-git-src/sonic.c.o");
        if !sonic_lib.exists() && sonic_obj.exists() {
            std::process::Command::new("ar")
                .args(["rcs", sonic_lib.to_str().unwrap(), sonic_obj.to_str().unwrap()])
                .status()
                .expect("Failed to create libsonic.a");
        }

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

        // Build libopus.a from source if it doesn't exist
        let opus_src = std::path::Path::new("../../third_party/opus-src");
        let opus_build = opus_src.join("build");
        let opus_lib = opus_build.join("libopus.a");
        if !opus_lib.exists() && opus_src.join("CMakeLists.txt").exists() {
            std::fs::create_dir_all(&opus_build).expect("Failed to create opus build dir");
            let status = std::process::Command::new("cmake")
                .args([
                    "-B", opus_build.to_str().unwrap(),
                    "-S", opus_src.to_str().unwrap(),
                    "-DCMAKE_BUILD_TYPE=Release",
                    "-DBUILD_SHARED_LIBS=OFF",
                ])
                .status()
                .expect("Failed to configure opus");
            assert!(status.success(), "CMake configure for opus failed");

            let status = std::process::Command::new("cmake")
                .args(["--build", opus_build.to_str().unwrap(), "--config", "Release"])
                .status()
                .expect("Failed to build opus");
            assert!(status.success(), "CMake build for opus failed");
        }

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
