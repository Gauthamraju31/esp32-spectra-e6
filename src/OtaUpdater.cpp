#include "OtaUpdater.h"

#include <HTTPClient.h>
#include <Update.h>
#include <WiFiClientSecure.h>
#include <WiFiClient.h>
#include <nvs.h>

#include "ApplicationConfig.h"

const char* OTA_NVS_NAMESPACE = "ota_storage";
const char* OTA_ETAG_KEY = "ota_etag";

String OtaUpdater::getStoredETag() {
  nvs_handle_t nvsHandle;
  esp_err_t err = nvs_open(OTA_NVS_NAMESPACE, NVS_READONLY, &nvsHandle);
  if (err != ESP_OK) return "";

  size_t requiredSize = 0;
  err = nvs_get_str(nvsHandle, OTA_ETAG_KEY, NULL, &requiredSize);
  if (err != ESP_OK || requiredSize == 0) {
    nvs_close(nvsHandle);
    return "";
  }

  char* etag = new char[requiredSize];
  nvs_get_str(nvsHandle, OTA_ETAG_KEY, etag, &requiredSize);
  nvs_close(nvsHandle);

  String result = String(etag);
  delete[] etag;
  return result;
}

void OtaUpdater::storeETag(const String& etag) {
  nvs_handle_t nvsHandle;
  esp_err_t err = nvs_open(OTA_NVS_NAMESPACE, NVS_READWRITE, &nvsHandle);
  if (err != ESP_OK) return;

  nvs_set_str(nvsHandle, OTA_ETAG_KEY, etag.c_str());
  nvs_commit(nvsHandle);
  nvs_close(nvsHandle);
  Serial.println("Stored new OTA ETag: " + etag);
}

void OtaUpdater::checkForUpdate() {
  String otaUrl = String(DEFAULT_OTA_URL);
  if (otaUrl.length() == 0) {
    Serial.println("No OTA URL configured, skipping OTA check.");
    return;
  }

  Serial.println("Checking for OTA update at: " + otaUrl);

  std::unique_ptr<WiFiClientSecure> secureClient;
  std::unique_ptr<WiFiClient> plainClient;
  HTTPClient http;

  if (otaUrl.startsWith("https://")) {
    secureClient.reset(new WiFiClientSecure());
    secureClient->setInsecure();
    http.begin(*secureClient, otaUrl);
  } else {
    plainClient.reset(new WiFiClient());
    http.begin(*plainClient, otaUrl);
  }

  http.setTimeout(15000); // Wait up to 15s for Cloudflare R2

  String storedETag = getStoredETag();
  if (storedETag.length() > 0) {
    http.addHeader("If-None-Match", storedETag);
    Serial.println("Using stored OTA ETag: " + storedETag);
  }

  const char* headerKeys[] = {"ETag", "Content-Length"};
  http.collectHeaders(headerKeys, 2);

  int httpCode = http.GET();

  if (httpCode == HTTP_CODE_NOT_MODIFIED) {
    Serial.println("Firmware not modified (304), bypassing OTA");
    http.end();
    return;
  }

  if (httpCode != HTTP_CODE_OK) {
    Serial.printf("OTA check failed, HTTP code: %d\n", httpCode);
    http.end();
    return;
  }

  int contentLength = http.getSize();
  if (contentLength <= 0) {
    Serial.println("OTA check failed, unknown content length");
    http.end();
    return;
  }

  String newETag = http.header("ETag");
  Serial.printf("New firmware found. Size: %d, ETag: %s\n", contentLength, newETag.c_str());
  Serial.println("Starting OTA update... DO NOT UNPLUG POWER!");

  // Use U_FLASH to update the program memory
  bool canBegin = Update.begin(contentLength, U_FLASH);
  if (!canBegin) {
    Serial.printf("Not enough space to start OTA, Error: %d\n", Update.getError());
    http.end();
    return;
  }

  WiFiClient* stream = http.getStreamPtr();
  size_t written = Update.writeStream(*stream);

  if (written == contentLength) {
    Serial.println("OTA bytes written: " + String(written));
  } else {
    Serial.printf("OTA Error: Written %d / %d bytes. Aborting.\n", written, contentLength);
    Update.abort();
    http.end();
    return;
  }

  if (Update.end()) {
    if (Update.isFinished()) {
      Serial.println("OTA Update successfully completed!");
      if (newETag.length() > 0) {
        storeETag(newETag);
      }
      Serial.println("Rebooting device to apply new firmware...");
      Serial.flush();
      delay(1000);
      ESP.restart();
    } else {
      Serial.println("OTA Update fell short! Something went wrong.");
    }
  } else {
    Serial.printf("OTA Error Occurred. Error code: %d\n", Update.getError());
  }

  http.end();
}
