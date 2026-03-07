#ifndef DISPLAY_TYPE_H
#define DISPLAY_TYPE_H

#include <GxEPD2_7C.h>

#include "GxEPD2_400c_GDEP040E01.h"

// Display instance for 4" Waveshare 27367 (Spectra 6 E Ink, GDEP040E01, 600x400, 6-color)
using DisplayType = GxEPD2_7C<GxEPD2_400c_GDEP040E01, GxEPD2_400c_GDEP040E01::HEIGHT>;
using Epd2Type = GxEPD2_400c_GDEP040E01;

#endif
