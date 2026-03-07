# ESP32 Spectra E6 - E-Paper Image Display

An ESP32 firmware for displaying images on e-paper displays with automatic dithering and color optimization. The device
fetches images from a configurable URL, processes them through a dithering service for optimal e-paper display, and
shows them with efficient power management.

![FinishedDevice](https://blog.shvn.dev/posts/2025-esp32-spectra-e6/cover_hu_598c6daf125e5264.jpg)

See also the [blog post](https://blog.shvn.dev/posts/2025-esp32-spectra-e6/) for more details about this project.

## Features

- Image fetching and display from configurable URLs
- Automatic image dithering and color optimization for e-paper displays
- Configuration via web interface (captive portal)
- Deep sleep mode for battery efficiency
- ETag-based caching to avoid unnecessary downloads
- Battery level monitoring

## Image Processing

- **Image dithering**: Processed through dithering service
- **Color palette**: Optimized for 6-color e-paper displays (black, white, yellow, red, blue, green)
- **Resolution**: Automatically scaled to match display dimensions

## Hardware Required

- **ESP32 Development Board**: [LilyGO T7-S3](https://lilygo.cc/products/t7-s3) - or any other ESP32 board
- **E-Paper Display**: Compatible with 6-color e-paper displays such as
  [E Ink Spectra E6](https://www.waveshare.com/product/displays/e-paper/epaper-1/7.3inch-e-paper-hat-e.htm)
- **Battery**: 3.7V LiPo battery
- **WiFi Network**: For initial configuration and image downloads
- **Mobile Device**: For connecting to configuration interface

## Setup

1. Install PlatformIO
2. Clone this repository
3. **Configure display type & features** (if necessary):
   - Edit `src/config_default.h` to change `USE_EXTERNAL_DITHER_SERVICE` if you want to use the public `dither.shvn.dev` API instead of the local server
   - Edit `src/DisplayType.h` to match your specific e-paper display
   - Update pin definitions in `include/boards.h` if needed
4. Build and upload the firmware:
   ```bash
   pio run --target upload
   ```
5. Upload the filesystem image (contains web interface files):
   ```bash
   pio run --target uploadfs
   ```

## Configuration

1. Power on the device (shows configuration screen with QR code on first boot)
2. Scan the QR code with your phone to connect to the device's WiFi hotspot
3. Your web browser will open automatically showing the configuration page
4. Configure:
   - **WiFi credentials**: Your home network SSID and password
   - **Image URL**: Direct URL to the image you want to display

Configuration mode will automatically activate if WiFi connection fails or credentials are invalid.

## Usage

- **Auto-refresh**: Device periodically downloads and displays the configured image, then enters deep sleep
- **Configuration mode**: Automatically shown when no WiFi credentials are configured or WiFi connection fails
- **Battery monitoring**: Current battery level is displayed on screen
- **ETag caching**: Only downloads new images when they've changed (saves bandwidth and battery)

## Server

The `server/` directory contains a Go web server that provides a web UI for generating images, dithering them for the e-paper display, and serving them to the ESP32.

### Image Providers

| Provider | `IMAGE_PROVIDER` | Description |
|----------|-----------------|-------------|
| **Stub** (default) | `stub` | Fetches random photos from [picsum.photos](https://picsum.photos/) — useful for testing the pipeline without an API key |
| **OpenAI** | `openai` | Generates images via OpenAI's DALL-E API |
| **NanoBanana** | `nanobanana` | Generates images via the NanoBanana API |

### Running the Server

```bash
cd server
go build -o server .
PASSWORD=your_password ./server
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `PASSWORD` | *(required)* | Access password for the web UI |
| `IMAGE_PROVIDER` | `stub` | Image provider (`stub`, `openai`, `nanobanana`) |
| `OPENAI_API_KEY` | — | Required when `IMAGE_PROVIDER=openai` |
| `NANOBANANA_API_KEY` | — | Required when `IMAGE_PROVIDER=nanobanana` |
| `NANOBANANA_URL` | `https://api.nanobanana.com` | NanoBanana API endpoint |
| `DITHER_SERVICE_URL` | `https://dither.shvn.dev` | Remote dither service URL |
| `DITHER_MODE` | `local_with_fallback` | `local`, `remote`, or `local_with_fallback` |
| `DAILY_RATE_LIMIT` | `10` | Max image generations per day |
| `DISPLAY_WIDTH` | `600` | E-paper display width in pixels |
| `DISPLAY_HEIGHT` | `400` | E-paper display height in pixels |
| `DATA_DIR` | `./data` | Directory for storing generated images |
| `S3_ENDPOINT` | — | Target URL for S3/R2 storage (e.g. `https://<hash>.r2.cloudflarestorage.com`) |
| `S3_ACCESS_KEY` | — | S3 compatible access key |
| `S3_SECRET_KEY` | — | S3 compatible secret key |
| `S3_BUCKET_NAME` | — | The bucket name to upload dithered and preview images into |

### Web UI Features

- **Prompt-based generation**: Enter a text prompt to generate an image
- **Orientation toggle**: Switch between landscape and portrait modes
- **Side-by-side preview**: View both the original and dithered (e-paper) images
### Deploying to Google Cloud Run (Automated via GitHub Actions)

The server is containerized and supports automated deployment to Google Cloud Run via GitHub Actions.

1. Create a Google Cloud Project and enable the **Cloud Run API** and **Artifact Registry API**.
2. Create an Artifact Registry Docker repository named `esp32-spectra` in `us-central1`.
3. Create a Service Account with the following roles:
   - `Cloud Run Admin`
   - `Artifact Registry Writer`
   - `Service Account User` (required to act as the compute service account)
4. Generate a JSON key for this Service Account.
5. In your GitHub repository, go to **Settings > Secrets and variables > Actions** and add:
   - `GCP_PROJECT_ID`: Your Google Cloud project ID (e.g. `my-awesome-project-123456`).
   - `GCP_CREDENTIALS`: Paste the contents of the Service Account JSON key.
   - `APP_PASSWORD`: The password you want to use for the web UI.

Any push to the `main` branch that modifies files in the `server/` directory will automatically build the Docker image, push it to Artifact Registry, and deploy it to Cloud Run. The server automatically detects the Cloud Run environment and uses its temporary filesystem (`/tmp/esp32-data`) for image caching.

