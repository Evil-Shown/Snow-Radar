import 'package:flutter/material.dart';

/// Snow Radar visual identity.
///
/// Aesthetic direction: dark-cinematic with Sri Lankan ancient-civilization
/// motifs (Sigiriya fresco pigments, moonstone, granite) — NOT generic
/// Material defaults. Colors reference: night granite, Sigiriya gold-ochre,
/// moonstone teal used for "stealth" states.
class SnowRadarTheme {
  static const _graniteBlack = Color(0xFF0B0D10);
  static const _graniteSlate = Color(0xFF151922);
  static const _sigiriyaGold = Color(0xFFC9A227);
  static const _moonstone = Color(0xFF4FB3A9);
  static const _frescoClay = Color(0xFFB3563E);
  static const _mistText = Color(0xFFE8E6E1);
  static const _mistDim = Color(0xFF8B9099);

  static ThemeData dark() {
    final scheme = ColorScheme.dark(
      surface: _graniteBlack,
      primary: _sigiriyaGold,
      secondary: _moonstone,
      error: _frescoClay,
      onSurface: _mistText,
      onPrimary: _graniteBlack,
    );
    return ThemeData(
      useMaterial3: true,
      colorScheme: scheme,
      scaffoldBackgroundColor: _graniteBlack,
      appBarTheme: const AppBarTheme(
        backgroundColor: _graniteBlack,
        elevation: 0,
        centerTitle: true,
        titleTextStyle: TextStyle(
          color: _mistText,
          fontSize: 18,
          fontWeight: FontWeight.w600,
          letterSpacing: 2.5,
        ),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: _sigiriyaGold,
          foregroundColor: _graniteBlack,
          minimumSize: const Size.fromHeight(56),
          shape: const RoundedRectangleBorder(borderRadius: BorderRadius.zero),
          textStyle: const TextStyle(fontWeight: FontWeight.w700, letterSpacing: 1.2),
        ),
      ),
      cardTheme: CardTheme(
        color: _graniteSlate,
        elevation: 0,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(4)),
        margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      ),
      dividerTheme: DividerThemeData(color: _mistDim.withValues(alpha: 0.15)),
    );
  }
}
