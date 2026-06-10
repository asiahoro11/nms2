import 'package:flutter/material.dart';

import '../api/nms_api_client.dart';
import '../models/nms_models.dart';
import '../storage/session_store.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({required this.onLogin, super.key});

  final ValueChanged<NmsSession> onLogin;

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _formKey = GlobalKey<FormState>();
  final _sessionStore = SessionStore();
  final _siteController = TextEditingController();
  final _hostController = TextEditingController();
  final _usernameController = TextEditingController();
  final _passwordController = TextEditingController();
  final _twoFactorController = TextEditingController();

  bool _busy = false;
  String? _error;
  LoginResult? _pendingTwoFactor;

  @override
  void initState() {
    super.initState();
    _loadDefaults();
  }

  @override
  void dispose() {
    _siteController.dispose();
    _hostController.dispose();
    _usernameController.dispose();
    _passwordController.dispose();
    _twoFactorController.dispose();
    super.dispose();
  }

  Future<void> _loadDefaults() async {
    final siteName = await _sessionStore.loadLastSiteName();
    final host = await _sessionStore.loadLastHost();
    if (!mounted) {
      return;
    }
    _siteController.text = siteName;
    _hostController.text = host;
  }

  Future<void> _login() async {
    if (!_formKey.currentState!.validate()) {
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });

    final baseUrl = NmsApiClient.normalizeBaseUrl(_hostController.text);
    final siteName = _siteController.text.trim().isEmpty ? 'NMS Site' : _siteController.text.trim();
    final client = NmsApiClient(baseUrl: baseUrl);

    try {
      await client.getSystemInfo();
      final result = await client.login(
        username: _usernameController.text.trim(),
        password: _passwordController.text,
      );
      if (!mounted) {
        return;
      }
      if (result.requiresTwoFactor) {
        setState(() {
          _pendingTwoFactor = result;
          _busy = false;
        });
        return;
      }
      await _completeLogin(baseUrl, siteName, result);
    } on NmsApiException catch (error) {
      _setError(error.message);
    } on Object {
      _setError('Login failed.');
    }
  }

  Future<void> _verifyTwoFactor() async {
    final pending = _pendingTwoFactor;
    if (pending == null) {
      return;
    }
    final code = _twoFactorController.text.trim();
    if (code.isEmpty) {
      _setError('Enter the verification code.');
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
    });

    final baseUrl = NmsApiClient.normalizeBaseUrl(_hostController.text);
    final siteName = _siteController.text.trim().isEmpty ? 'NMS Site' : _siteController.text.trim();
    final client = NmsApiClient(baseUrl: baseUrl);

    try {
      final result = await client.verifyTwoFactor(
        code: code,
        challengeToken: pending.challengeToken ?? '',
        username: _usernameController.text.trim(),
        method: pending.twoFactorMethod,
      );
      await _completeLogin(baseUrl, siteName, result);
    } on NmsApiException catch (error) {
      _setError(error.message);
    } on Object {
      _setError('Verification failed.');
    }
  }

  Future<void> _completeLogin(
    String baseUrl,
    String siteName,
    LoginResult result,
  ) async {
    if (!result.hasToken) {
      _setError('NMS did not return a session token.');
      return;
    }
    final session = NmsSession(
      siteName: siteName,
      baseUrl: baseUrl,
      token: result.token,
      expiresAt: result.expiresAt,
      user: result.user,
    );
    await widget.onLogin(session);
  }

  void _setError(String message) {
    if (!mounted) {
      return;
    }
    setState(() {
      _error = message;
      _busy = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    final pendingTwoFactor = _pendingTwoFactor != null;

    return Scaffold(
      appBar: AppBar(title: const Text('NMS Mobile')),
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 520),
            child: ListView(
              padding: const EdgeInsets.all(20),
              children: [
                Text(
                  pendingTwoFactor ? 'Two-factor verification' : 'Connect to NMS',
                  style: Theme.of(context).textTheme.headlineMedium,
                ),
                const SizedBox(height: 8),
                Text(
                  pendingTwoFactor
                      ? 'Enter the code from your configured second factor.'
                      : 'Use the host address, username, and password for the existing NMS server.',
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                const SizedBox(height: 24),
                Form(
                  key: _formKey,
                  child: Column(
                    children: [
                      if (!pendingTwoFactor) ...[
                        TextFormField(
                          controller: _siteController,
                          textInputAction: TextInputAction.next,
                          decoration: const InputDecoration(
                            labelText: 'Site name',
                            hintText: 'Main office NMS',
                          ),
                        ),
                        const SizedBox(height: 12),
                        TextFormField(
                          controller: _hostController,
                          textInputAction: TextInputAction.next,
                          keyboardType: TextInputType.url,
                          decoration: const InputDecoration(
                            labelText: 'NMS host',
                            hintText: 'https://nms.example.com:8080',
                          ),
                          validator: (value) {
                            if (value == null || value.trim().isEmpty) {
                              return 'Enter the NMS domain or IP.';
                            }
                            return null;
                          },
                        ),
                        const SizedBox(height: 12),
                        TextFormField(
                          controller: _usernameController,
                          textInputAction: TextInputAction.next,
                          decoration: const InputDecoration(labelText: 'Username'),
                          validator: (value) {
                            if (value == null || value.trim().isEmpty) {
                              return 'Enter the username.';
                            }
                            return null;
                          },
                        ),
                        const SizedBox(height: 12),
                        TextFormField(
                          controller: _passwordController,
                          obscureText: true,
                          decoration: const InputDecoration(labelText: 'Password'),
                          validator: (value) {
                            if (value == null || value.isEmpty) {
                              return 'Enter the password.';
                            }
                            return null;
                          },
                          onFieldSubmitted: (_) => _busy ? null : _login(),
                        ),
                      ] else ...[
                        TextFormField(
                          controller: _twoFactorController,
                          autofocus: true,
                          keyboardType: TextInputType.number,
                          decoration: const InputDecoration(labelText: 'Verification code'),
                          onFieldSubmitted: (_) => _busy ? null : _verifyTwoFactor(),
                        ),
                      ],
                    ],
                  ),
                ),
                if (_error != null) ...[
                  const SizedBox(height: 16),
                  _ErrorBanner(message: _error!),
                ],
                const SizedBox(height: 24),
                FilledButton(
                  onPressed: _busy ? null : pendingTwoFactor ? _verifyTwoFactor : _login,
                  child: _busy
                      ? const SizedBox.square(
                          dimension: 20,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : Text(pendingTwoFactor ? 'Verify' : 'Login'),
                ),
                if (pendingTwoFactor) ...[
                  const SizedBox(height: 12),
                  TextButton(
                    onPressed: _busy
                        ? null
                        : () {
                            setState(() {
                              _pendingTwoFactor = null;
                              _twoFactorController.clear();
                              _error = null;
                            });
                          },
                    child: const Text('Back'),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _ErrorBanner extends StatelessWidget {
  const _ErrorBanner({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.errorContainer,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Text(
          message,
          style: TextStyle(color: Theme.of(context).colorScheme.onErrorContainer),
        ),
      ),
    );
  }
}
