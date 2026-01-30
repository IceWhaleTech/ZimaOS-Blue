fn main() {
    // Platform-specific build configuration
    #[cfg(target_os = "macos")]
    {
        // Link the Go static library (libecho.a)
        // The library should be placed in tauri-app/src-tauri/lib/
        println!("cargo:rustc-link-search=native=lib");
        println!("cargo:rustc-link-lib=static=echo");

        // Link required system frameworks for Go runtime
        println!("cargo:rustc-link-lib=framework=CoreFoundation");
        println!("cargo:rustc-link-lib=framework=Security");
        println!("cargo:rustc-link-lib=framework=SystemConfiguration");
        println!("cargo:rustc-link-lib=framework=IOKit");

        // Link Go runtime dependencies
        println!("cargo:rustc-link-lib=resolv");

        // Rerun if the library changes
        println!("cargo:rerun-if-changed=lib/libecho.a");
    }

    tauri_build::build()
}
