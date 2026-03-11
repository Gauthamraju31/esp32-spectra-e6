#ifndef CONFIG_DEFAULT_H
#define CONFIG_DEFAULT_H

// Default configuration values (safe to commit to repository)
// For development, create config_dev.h with your actual credentials

const char DEFAULT_WIFI_SSID[] = "";
const char DEFAULT_WIFI_PASSWORD[] = "";
const char DEFAULT_IMAGE_URL[] = "";
const char DEFAULT_OTA_URL[] = "";

// const char DEFAULT_WIFI_SSID[] = "GauthamWifi";
// const char DEFAULT_WIFI_PASSWORD[] = "puduhatty";
// // const char DEFAULT_IMAGE_URL[] = "http://192.168.1.100:8080/image";
// const char DEFAULT_IMAGE_URL[] = "http://192.168.1.100:8080/image";

// Whether to use an external dithering service (dither.shvn.dev) or fetch the BMP directly.
// Set to false to fetch the already-dithered BMP directly from the configured imageUrl (e.g. your local server).
// Set to true to append dithering parameters and send the URL to the external service.
#define USE_EXTERNAL_DITHER_SERVICE false

#endif  // CONFIG_DEFAULT_H