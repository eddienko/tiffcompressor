# TIFFCompressor

**TIFFCompressor** is a fast, multi-threaded Go program that compresses `.tif` and `.tiff` image files using Deflate compression. It can either compress files in place or mirror the original directory structure into a separate output directory.

## 🚀 Features

- Multi-threaded compression (configurable with `--threads`)
- Optional output directory that mirrors input structure
- Total compression summary: size before, after, and saved
- Progress bar and detailed logging to file
- Cross-platform: works on Linux, macOS, and Windows
- Pure Go implementation (no external C dependencies)

## 📦 Installation

You need [Go](https://golang.org/dl/) installed (version 1.18 or later recommended).

```bash
git clone https://github.com/yourusername/tiffcompressor.git
cd tiffcompressor
go build -o tiffcompressor
```


## 🧪 Usage

```bash
./tiffcompressor [OPTIONS] <input_directory>
```

### Required:

  * ``<input_directory>``: Path to a directory containing .tif or .tiff files. The tool will recursively compress all such files.

**Options:**


| Flag        | Description                                                       | Default                   |
| ----------- | ----------------------------------------------------------------- | ------------------------- |
| `--threads` | Number of concurrent workers to use for compression               | Number of CPU cores       |
| `--logfile` | Path to a log file that records per-file stats and errors         | `compression.log`         |
| `--outdir`  | Optional path to mirror the input directory with compressed files | (none, compress in place) |

## 🧰 Examples

### Compress in place:

```bash
./tiffcompressor --threads=8 ./images
```

This will compress all .tif and .tiff files found under ./images, overwriting them in place.

### Compress into mirrored output directory:

```bash
./tiffcompressor --threads=4 --logfile=compress.log --outdir=./compressed ./images
```

This will compress all images under ./images and save them under:

```bash
./compressed/images/
```

The full directory tree will be preserved and the files in the input directory will not be modified.

Note that in windows this should be run as 

```bash
.\tiffcompressor.exe C:\images --outdir=E:\compressed
```

## 📊 Output

The tool prints a progress bar and, upon completion, outputs a summary like:


```bash
✅ All files processed.
💾 Total original size: 154.23 MB
📦 Total compressed size: 32.91 MB
📉 Space saved: 121.32 MB (78.7%)
```

The ``--logfile ``(default: compression.log) also contains per-file compression stats and error messages.

# 📄 License

MIT License

# ✨ Author

Created by Eduardo Gonzalez-Solares. Contributions welcome!
