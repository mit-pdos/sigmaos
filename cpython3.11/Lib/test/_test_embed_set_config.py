# bpo-42260: Test _PyInterpreterState_GetConfigCopy()
# and _PyInterpreterState_SetConfig().
#
# Test run in a subprocess since set_config(get_config())
# does reset sys attributes to their state of the Python startup
# (before the site module is run).

import _testinternalcapi
import sys
import unittest
from test import support
from test.support import MS_WINDOWS


MAX_HASH_SEED = 4294967295


BOOL_OPTIONS = [
    'isolated',
    'use_environment',
    'dev_mode',
    'install_signal_handlers',
    'use_hash_seed',
    'faulthandler',
    'import_time',
    'code_debug_ranges',
    'show_ref_count',
    'dump_refs',
    'malloc_stats',
    'parse_argv',
    'site_import',
    'warn_default_encoding',
    'inspect',
    'interactive',
    'parser_debug',
    'write_bytecode',
    'quiet',
    'user_site_directory',
    'configure_c_stdio',
    'buffered_stdio',
    'use_frozen_modules',
    'safe_path',
    'pathconfig_warnings',
    'module_search_paths_set',
    'skip_source_first_line',
    '_install_importlib',
    '_init_main',
    '_is_python_build',
]
if MS_WINDOWS:
    BOOL_OPTIONS.append('legacy_windows_stdio')


class SetConfigTests(unittest.TestCase):
    def setUp(self):
        self.old_config = _testinternalcapi.get_config()
        self.sys_copy = dict(sys.__dict__)

    def tearDown(self):
        _testinternalcapi.reset_path_config()
        _testinternalcapi.set_config(self.old_config)
        sys.__dict__.clear()
        sys.__dict__.update(self.sys_copy)

    def set_config(self, **kwargs):
        _testinternalcapi.set_config(self.old_config | kwargs)

    def check(self, **kwargs):
        self.set_config(**kwargs)
        for key, value in kwargs.items():
            self.assertEqual(getattr(sys, key), value,
                             (key, value))

    def test_set_invalid(self):
        invalid_uint = -1
        NULL = None
        invalid_wstr = NULL
        # PyWideStringList strings must be non-NULL
        invalid_wstrlist = ["abc", NULL, "def"]

        type_tests = []
        value_tests = [
            # enum
            ('_config_init', 0),
            ('_config_init', 4),
            # unsigned long
            ("hash_seed", -1),
            ("hash_seed", MAX_HASH_SEED + 1),
        ]

        # int (unsigned)
        int_options = [
            '_config_init',
            'bytes_warning',
            'optimization_level',
            'tracemalloc',
            'verbose',
        ]
        int_options.extend(BOOL_OPTIONS)
        for key in int_options:
            value_tests.append((key, invalid_uint))
            type_tests.append((key, "abc"))
            type_tests.append((key, 2.0))

        # wchar_t*
        for key in (
            'filesystem_encoding',
            'filesystem_errors',
            'stdio_encoding',
            'stdio_errors',
            'check_hash_pycs_mode',
            'program_name',
            'platlibdir',
            # optional wstr:
            # 'pythonpath_env'
            # 'home'
            # 'pycache_prefix'
            # 'run_command'
            # 'run_module'
            # 'run_filename'
            # 'executable'
            # 'prefix'
            # 'exec_prefix'
            # 'base_executable'
            # 'base_prefix'
            # 'base_exec_prefix'
        ):
            value_tests.append((key, invalid_wstr))
            type_tests.append((key, b'bytes'))
            type_tests.append((key, 123))

        # PyWideStringList
        for key in (
            'orig_argv',
            'argv',
            'xoptions',
            'warnoptions',
            'module_search_paths',
        ):
            if key != 'xoptions':
                value_tests.append((key, invalid_wstrlist))
            type_tests.append((key, 123))
            type_tests.append((key, "abc"))
            type_tests.append((key, [123]))
            type_tests.append((key, [b"bytes"]))


        if MS_WINDOWS:
            value_tests.append(('legacy_windows_stdio', invalid_uint))

        for exc_type, tests in (
            (ValueError, value_tests),
            (TypeError, type_tests),
        ):
            for key, value in tests:
                config = self.old_config | {key: value}
                with self.subTest(key=key, value=value, exc_type=exc_type):
                    with self.assertRaises(exc_type):
                        _testinternalcapi.set_config(config)

    def test_flags(self):
        bool_options = set(BOOL_OPTIONS)
        for sys_attr, key, value in (
            ("debug", "parser_debug", 2),
            ("inspect", "inspect", 3),
            ("interactive", "interactive", 4),
            ("optimize", "optimization_level", 5),
            ("verbose", "verbose", 6),
            ("bytes_warning", "bytes_warning", 7),
            ("quiet", "quiet", 8),
            ("isolated", "isolated", 9),
        ):
            with self.subTest(sys=sys_attr, key=key, value=value):
                self.set_config(**{key: value, 'parse_argv': 0})
                if key in bool_options:
                    self.assertEqual(getattr(sys.flags, sys_attr), int(bool(value)))
                else:
                    self.assertEqual(getattr(sys.flags, sys_attr), value)

        self.set_config(write_bytecode=0)
        self.assertEqual(sys.flags.dont_write_bytecode, True)
        self.assertEqual(sys.dont_write_bytecode, True)

        self.set_config(write_bytecode=1)
        self.assertEqual(sys.flags.dont_write_bytecode, False)
        self.assertEqual(sys.dont_write_bytecode, False)

        self.set_config(user_site_directory=0, isolated=0)
        self.assertEqual(sys.flags.no_user_site, 1)
        self.set_config(user_site_directory=1, isolated=0)
        self.assertEqual(sys.flags.no_user_site, 0)

        self.set_config(site_import=0)
        self.assertEqual(sys.flags.no_site, 1)
        self.set_config(site_import=1)
        self.assertEqual(sys.flags.no_site, 0)

        self.set_config(dev_mode=0)
        self.assertEqual(sys.flags.dev_mode, False)
        self.set_config(dev_mode=1)
        self.assertEqual(sys.flags.dev_mode, True)

        self.set_config(use_environment=0, isolated=0)
        self.assertEqual(sys.flags.ignore_environment, 1)
        self.set_config(use_environment=1, isolated=0)
        self.assertEqual(sys.flags.ignore_environment, 0)

        self.set_config(use_hash_seed=1, hash_seed=0)
        self.assertEqual(sys.flags.hash_randomization, 0)
        self.set_config(use_hash_seed=0, hash_seed=0)
        self.assertEqual(sys.flags.hash_randomization, 1)
        self.set_config(use_hash_seed=1, hash_seed=123)
        self.assertEqual(sys.flags.hash_randomization, 1)

        if support.Py_GIL_DISABLED:
            self.set_config(enable_gil=-1)
            self.assertEqual(sys.flags.gil, None)
            self.set_config(enable_gil=0)
            self.assertEqual(sys.flags.gil, 0)
            self.set_config(enable_gil=1)
            self.assertEqual(sys.flags.gil, 1)
        else:
            # Builds without Py_GIL_DISABLED don't have
            # PyConfig.enable_gil. sys.flags.gil is always defined to 1, for
            # consistency.
            self.assertEqual(sys.flags.gil, 1)

    def test_options(self):
        self.check(warnoptions=[])
        self.check(warnoptions=["default", "ignore"])

        self.set_config(xoptions={})
        self.assertEqual(sys._xoptions, {})
        self.set_config(xoptions={"dev": True, "tracemalloc": "5"})
        self.assertEqual(sys._xoptions, {"dev": True, "tracemalloc": "5"})

    def test_pathconfig(self):
        self.check(
            executable='executable',
            prefix="prefix",
            base_prefix="base_prefix",
            exec_prefix="exec_prefix",
            base_exec_prefix="base_exec_prefix",
            platlibdir="platlibdir")

        self.set_config(base_executable="base_executable")
        self.assertEqual(sys._base_executable, "base_executable")

        # When base_xxx is NULL, value is copied from xxxx
        self.set_config(
            executable='executable',
            prefix="prefix",
            exec_prefix="exec_prefix",
            base_executable=None,
            base_prefix=None,
            base_exec_prefix=None)
        self.assertEqual(sys._base_executable, "executable")
        self.assertEqual(sys.base_prefix, "prefix")
        self.assertEqual(sys.base_exec_prefix, "exec_prefix")

    def test_path(self):
        self.set_config(module_search_paths_set=1,
                        module_search_paths=['a', 'b', 'c'])
        self.assertEqual(sys.path, ['a', 'b', 'c'])

        # sys.path is reset if module_search_paths_set=0
        self.set_config(module_search_paths_set=0,
                        module_search_paths=['new_path'])
        self.assertNotEqual(sys.path, ['a', 'b', 'c'])
        self.assertNotEqual(sys.path, ['new_path'])

    def test_argv(self):
        self.set_config(parse_argv=0,
                        argv=['python_program', 'args'],
                        orig_argv=['orig', 'orig_args'])
        self.assertEqual(sys.argv, ['python_program', 'args'])
        self.assertEqual(sys.orig_argv, ['orig', 'orig_args'])

        self.set_config(parse_argv=0,
                        argv=[],
                        orig_argv=[])
        self.assertEqual(sys.argv, [''])
        self.assertEqual(sys.orig_argv, [])

    def test_pycache_prefix(self):
        self.check(pycache_prefix=None)
        self.check(pycache_prefix="pycache_prefix")


if __name__ == "__main__":
    unittest.main()
