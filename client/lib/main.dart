import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'state/connection_controller.dart';
import 'theme/snowradar_theme.dart';

void main() {
  runApp(const ProviderScope(child: SnowRadarApp()));
}

class SnowRadarApp extends StatelessWidget {
  const SnowRadarApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Snow Radar',
      theme: SnowRadarTheme.dark(),
      home: const HomeScreen(),
    );
  }
}

class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(connectionProvider);
    final controller = ref.read(connectionProvider.notifier);

    return Scaffold(
      appBar: AppBar(title: const Text('SNOW RADAR')),
      body: Column(
        children: [
          const Spacer(),
          _StatusGlyph(status: state.status),
          const SizedBox(height: 16),
          Text(
            switch (state.status) {
              TunnelStatus.connected =>
                'CONNECTED — ${state.stealthMode ? "STEALTH" : "STANDARD"}',
              TunnelStatus.connecting => 'ESTABLISHING TUNNEL…',
              TunnelStatus.failed => state.error ?? 'CONNECTION FAILED',
              TunnelStatus.disconnected => 'PROTECTED CHANNEL OFFLINE',
            },
            style: Theme.of(context).textTheme.titleMedium,
            textAlign: TextAlign.center,
          ),
          if (state.address != null)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Text(state.address!, style: const TextStyle(color: Color(0xFF8B9099))),
            ),
          const Spacer(),
          Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              children: [
                SwitchListTile(
                  title: const Text('Stealth Mode (AmneziaWG)'),
                  subtitle: const Text('Obfuscated handshake for DPI-heavy networks'),
                  value: state.stealthMode,
                  onChanged: controller.toggleStealth,
                ),
                Row(
                  children: [
                    Expanded(
                      child: RadioListTile<String>(
                        title: const Text('Singapore'),
                        value: 'sgp',
                        groupValue: state.nodeId,
                        onChanged: (v) => controller.selectNode(v!),
                      ),
                    ),
                    Expanded(
                      child: RadioListTile<String>(
                        title: const Text('Germany'),
                        value: 'fsn',
                        groupValue: state.nodeId,
                        onChanged: (v) => controller.selectNode(v!),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                FilledButton(
                  onPressed: state.status == TunnelStatus.connecting ? null : () {
                    // TODO(phase-6): wire real session token from secure storage.
                    controller.connect('PLACEHOLDER_TOKEN');
                  },
                  child: Text(switch (state.status) {
                    TunnelStatus.connected => 'DISCONNECT',
                    _ => 'CONNECT',
                  }),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _StatusGlyph extends StatelessWidget {
  const _StatusGlyph({required this.status});
  final TunnelStatus status;

  @override
  Widget build(BuildContext context) {
    final color = switch (status) {
      TunnelStatus.connected => const Color(0xFF4FB3A9), // moonstone
      TunnelStatus.connecting => const Color(0xFFC9A227), // sigiriya gold
      TunnelStatus.failed => const Color(0xFFB3563E),     // fresco clay
      TunnelStatus.disconnected => const Color(0xFF8B9099),
    };
    return Container(
      width: 132,
      height: 132,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: Border.all(color: color, width: 3),
      ),
      child: Icon(Icons.shield_outlined, size: 56, color: color),
    );
  }
}
