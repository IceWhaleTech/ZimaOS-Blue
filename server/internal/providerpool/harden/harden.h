#ifndef HARDEN_H
#define HARDEN_H

// harden_check returns 1 if a prior activation marker exists, 0 otherwise.
// sig/sig_len: license signature bytes used to derive a stable marker filename.
int harden_check(const char *data_dir, const unsigned char *sig, int sig_len);

// harden_mark writes an activation marker to a platform-specific hidden location.
// sig/sig_len: license signature bytes used to derive a stable marker filename.
void harden_mark(const char *data_dir, const unsigned char *sig, int sig_len);

#endif
