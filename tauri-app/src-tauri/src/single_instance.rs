// Windows single instance support using named mutex
use std::ptr;
use windows_sys::Win32::Foundation::{CloseHandle, HANDLE};
use windows_sys::Win32::System::Threading::{CreateMutexW, GetLastError, ERROR_ALREADY_EXISTS};

pub struct SingleInstance {
    mutex_handle: HANDLE,
}

impl SingleInstance {
    pub fn new(app_id: &str) -> Option<Self> {
        unsafe {
            // Create wide string for mutex name
            let mutex_name = format!("Global\\{}\0", app_id);
            let wide_name: Vec<u16> = mutex_name.encode_utf16().collect();

            // Try to create mutex
            let handle = CreateMutexW(ptr::null(), 0, wide_name.as_ptr());

            if handle == 0 {
                return None;
            }

            // Check if mutex already exists
            if GetLastError() == ERROR_ALREADY_EXISTS {
                CloseHandle(handle);
                return None;
            }

            Some(SingleInstance {
                mutex_handle: handle,
            })
        }
    }
}

impl Drop for SingleInstance {
    fn drop(&mut self) {
        unsafe {
            if self.mutex_handle != 0 {
                CloseHandle(self.mutex_handle);
            }
        }
    }
}
