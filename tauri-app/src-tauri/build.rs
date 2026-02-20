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

        // Conditionally link espeak-ng (only if built)
        let espeak_lib = std::path::Path::new("../../third_party/espeak-ng/build/src/libespeak-ng/libespeak-ng.a");
        if espeak_lib.exists() {
            println!("cargo:rustc-link-search=native=../../third_party/espeak-ng/build/src/libespeak-ng");
            println!("cargo:rustc-link-search=native=../../third_party/espeak-ng/build/src/ucd-tools");
            println!("cargo:rustc-link-search=native=../../third_party/espeak-ng/build/src/speechPlayer");
            println!("cargo:rustc-link-lib=static=espeak-ng");
            println!("cargo:rustc-link-lib=static=ucd");
            println!("cargo:rustc-link-lib=static=speechPlayer");
        }

        println!("cargo:rerun-if-changed=lib/libblue.a");
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
