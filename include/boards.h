
#pragma once

#if defined(BOARD_LILYGO_T7_S3)

// Pin definitions for LilyGO T7-S3 with Waveshare e-Paper HAT
#define EPD_CS 10    // Chip Select for SPI communication
#define EPD_DC 45    // Data/Command selection for the display
#define EPD_RSET 46  // Reset pin for the e-Paper display
#define EPD_BUSY 47  // Indicates when the display is busy

// SPI pins (ESP32-S3 hardware SPI)
#define EPD_MOSI 11    // SPI Data In (Master Out Slave In)
#define EPD_MISO (-1)  // Not used by e-paper
#define EPD_SCLK 12    // SPI Clock

// Battery monitoring
#define BATTERY_PIN 1  // ADC pin for battery monitoring (via voltage divider)

// LED (built-in on LilyGO T7-S3)
#define LED_PIN 17     // Built-in LED on GPIO17
#define LED_ON (HIGH)  // LED active high

#elif defined(BOARD_SEEED_XIAO_ESP32S3)

// Pin definitions for Seeed Studio XIAO ESP32S3 with Waveshare e-Paper HAT
#define EPD_CS 2    // Chip Select for SPI communication
#define EPD_DC 3    // Data/Command selection for the display
#define EPD_RSET 4  // Reset pin for the e-Paper display
#define EPD_BUSY 5  // Indicates when the display is busy
#define EPD_PWR 43  // Power control pin for e-Paper display

// SPI pins (XIAO ESP32S3 hardware SPI)
#define EPD_MOSI 9     // SPI Data In (Master Out Slave In)
#define EPD_MISO (-1)  // Not used by e-paper
#define EPD_SCLK 7     // SPI Clock

// Battery monitoring
#define BATTERY_PIN 1  // ADC pin for battery monitoring

// LED (built-in on XIAO ESP32S3)
#define LED_PIN 21     // Built-in LED on GPIO21
#define LED_ON (HIGH)  // LED active high

#else
  #error "Unknown board. Define BOARD_LILYGO_T7_S3 or BOARD_SEEED_XIAO_ESP32S3 in build flags."
#endif