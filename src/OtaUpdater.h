#pragma once

#include <Arduino.h>

class OtaUpdater {
 public:
  static void checkForUpdate();

 private:
  static String getStoredETag();
  static void storeETag(const String& etag);
};
