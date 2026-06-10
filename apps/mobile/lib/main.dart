import 'package:flutter/material.dart';

import 'src/api/nms_api_client.dart';
import 'src/models/nms_models.dart';
import 'src/screens/dashboard_screen.dart';
import 'src/screens/login_screen.dart';
import 'src/storage/session_store.dart';

void main() {
  runApp(const NmsMobileApp());
}

class NmsMobileApp extends StatefulWidget {
  const NmsMobileApp({super.key});

  @override
  State<NmsMobileApp> createState() => _NmsMobileAppState();
}

class _NmsMobileAppState extends State<NmsMobileApp> {
  final SessionStore _sessionStore = SessionStore();
  NmsSession? _session;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _restoreSession();
  }

  Future<void> _restoreSession() async {
    final session = await _sessionStore.loadSession();
    if (!mounted) {
      return;
    }
    setState(() {
      _session = session;
      _loading = false;
    });
  }

  Future<void> _handleLogin(NmsSession session) async {
    await _sessionStore.saveSession(session);
    if (!mounted) {
      return;
    }
    setState(() {
      _session = session;
    });
  }

  Future<void> _handleLogout() async {
    final session = _session;
    if (session != null) {
      final client = NmsApiClient(baseUrl: session.baseUrl, token: session.token);
      await client.logout();
    }
    await _sessionStore.clear();
    if (!mounted) {
      return;
    }
    setState(() {
      _session = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'NMS Mobile',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xff0f766e)),
        useMaterial3: true,
        inputDecorationTheme: const InputDecorationTheme(
          border: OutlineInputBorder(),
        ),
      ),
      home: _loading
          ? const _LoadingView()
          : _session == null
              ? LoginScreen(onLogin: _handleLogin)
              : DashboardScreen(session: _session!, onLogout: _handleLogout),
    );
  }
}

class _LoadingView extends StatelessWidget {
  const _LoadingView();

  @override
  Widget build(BuildContext context) {
    return const Scaffold(
      body: Center(child: CircularProgressIndicator()),
    );
  }
}
