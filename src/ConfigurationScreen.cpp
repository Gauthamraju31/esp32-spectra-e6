#include "ConfigurationScreen.h"

#include <memory>

#include "ConfigurationServer.h"
#include "HardwareSerial.h"

ConfigurationScreen::ConfigurationScreen(DisplayType& display)
    : display(display),
      accessPointName(ConfigurationServer::WIFI_AP_NAME),
      accessPointPassword(ConfigurationServer::WIFI_AP_PASSWORD) {
  gfx.begin(display);
}

String ConfigurationScreen::generateWiFiQRString() const {
  String wifiQRCodeString = "WIFI:T:WPA2;S:" + accessPointName + ";P:" + accessPointPassword + ";H:false;;";
  return wifiQRCodeString;
}

void ConfigurationScreen::drawQRCode(const String& wifiString, int x, int y, int scale) {
  const uint8_t qrCodeVersion4 = 4;
  uint8_t qrCodeDataBuffer[qrcode_getBufferSize(qrCodeVersion4)];
  QRCode qrCodeInstance;

  int qrGenerationResult =
      qrcode_initText(&qrCodeInstance, qrCodeDataBuffer, qrCodeVersion4, ECC_MEDIUM, wifiString.c_str());

  if (qrGenerationResult != 0) {
    Serial.print("Failed to generate QR code, error: ");
    Serial.println(qrGenerationResult);
    return;
  }

  for (uint8_t qrModuleY = 0; qrModuleY < qrCodeInstance.size; qrModuleY++) {
    for (uint8_t qrModuleX = 0; qrModuleX < qrCodeInstance.size; qrModuleX++) {
      bool moduleIsBlack = qrcode_getModule(&qrCodeInstance, qrModuleX, qrModuleY);
      if (moduleIsBlack) {
        display.fillRect(x + (qrModuleX * scale), y + (qrModuleY * scale), scale, scale, GxEPD_BLACK);
      }
    }
  }
}

void ConfigurationScreen::render() {
  Serial.println("Displaying configuration screen with QR code");

  display.init(115200);
  display.setRotation(ApplicationConfig::DISPLAY_ROTATION);
  Serial.printf("Display dimensions: %d x %d\n", display.width(), display.height());

  const int textLeftMargin = 20;
  const int lineSpacing = 40;
  const int qrCodeScale = 5;
  const int qrCodeModuleCount = 33;
  const int qrCodePixelSize = qrCodeModuleCount * qrCodeScale;  // 165px
  const int qrCodeQuietZone = 16;

  String wifiQRCodeString = generateWiFiQRString();

  bool isPortrait = display.height() > display.width();

  gfx.setFontMode(1);
  gfx.setBackgroundColor(GxEPD_BLUE);
  gfx.setForegroundColor(GxEPD_WHITE);

  display.setFullWindow();
  display.fillScreen(GxEPD_WHITE);

  // ── Header bar ────────────────────────────────────────────────────────
  display.fillRect(0, 0, display.width(), 70, GxEPD_BLUE);

  gfx.setFont(u8g2_font_open_iconic_embedded_4x_t);
  gfx.setCursor(textLeftMargin, 52);
  gfx.print((char)66);

  gfx.setFont(u8g2_font_fur17_tr);
  gfx.setCursor(textLeftMargin + 38, 47);
  gfx.print("Configuration Mode");

  gfx.setBackgroundColor(GxEPD_WHITE);
  gfx.setForegroundColor(GxEPD_BLACK);

  if (isPortrait) {
    // ── Portrait layout: instructions at top, QR code centred below ──────
    int currentY = 110;
    gfx.setFont(u8g2_font_fur14_tr);

    gfx.setCursor(textLeftMargin, currentY);
    gfx.print("1. Scan QR code below");
    currentY += lineSpacing;

    gfx.setCursor(textLeftMargin, currentY);
    gfx.print("2. Connect to WiFi:");
    currentY += 28;

    gfx.setFont(u8g2_font_courB14_tr);
    gfx.setCursor(textLeftMargin + 16, currentY);
    gfx.print(ConfigurationServer::WIFI_AP_NAME);
    currentY += lineSpacing;

    gfx.setFont(u8g2_font_fur14_tr);
    gfx.setCursor(textLeftMargin, currentY);
    gfx.print("3. Open browser & configure");
    currentY += lineSpacing;

    gfx.setCursor(textLeftMargin, currentY);
    gfx.print("4. Save settings and exit");
    currentY += lineSpacing + 10;

    // QR code centred horizontally below the instructions
    int qrCodeX = (display.width() - qrCodePixelSize) / 2;
    int qrCodeY = currentY;

    int qrBgSize = qrCodePixelSize + 2 * qrCodeQuietZone;
    int qrBgX = qrCodeX - qrCodeQuietZone;
    int qrBgY = qrCodeY - qrCodeQuietZone;

    display.fillRect(qrBgX - 4, qrBgY - 4, qrBgSize + 8, qrBgSize + 8, GxEPD_RED);
    display.fillRect(qrBgX, qrBgY, qrBgSize, qrBgSize, GxEPD_WHITE);
    drawQRCode(wifiQRCodeString, qrCodeX, qrCodeY, qrCodeScale);

  } else {
    // ── Landscape layout: text on left, QR code on right ─────────────────
    int qrCodeX = display.width() - qrCodePixelSize - qrCodeQuietZone - 30;
    int qrCodeY = (display.height() - qrCodePixelSize) / 2;

    int currentY = 130;
    gfx.setFont(u8g2_font_fur17_tr);

    gfx.setCursor(textLeftMargin, currentY);
    gfx.print("1. Scan QR code with your phone");
    currentY += lineSpacing;

    gfx.setCursor(textLeftMargin, currentY);
    gfx.print("2. Connect to WiFi network:");
    currentY += 25;

    gfx.setFont(u8g2_font_courB14_tr);
    gfx.setCursor(textLeftMargin + 30, currentY);
    gfx.print(ConfigurationServer::WIFI_AP_NAME);
    currentY += lineSpacing;

    gfx.setFont(u8g2_font_fur17_tr);
    gfx.setCursor(textLeftMargin, currentY);
    gfx.print("3. Open web browser and configure");
    currentY += lineSpacing;

    gfx.setCursor(textLeftMargin, currentY);
    gfx.print("4. Save settings and exit");

    int qrBgSize = qrCodePixelSize + 2 * qrCodeQuietZone;
    int qrBgX = qrCodeX - qrCodeQuietZone;
    int qrBgY = qrCodeY - qrCodeQuietZone;

    display.fillRect(qrBgX - 5, qrBgY - 5, qrBgSize + 10, qrBgSize + 10, GxEPD_RED);
    display.fillRect(qrBgX, qrBgY, qrBgSize, qrBgSize, GxEPD_WHITE);
    drawQRCode(wifiQRCodeString, qrCodeX, qrCodeY, qrCodeScale);
  }

  display.display();
  display.hibernate();

  Serial.println("Configuration screen rendered successfully");
}

int ConfigurationScreen::nextRefreshInSeconds() { return 600; }
