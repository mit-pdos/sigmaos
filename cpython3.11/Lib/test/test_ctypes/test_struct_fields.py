import unittest
from ctypes import Structure, Union, sizeof, c_char, c_int
from ._support import (CField, Py_TPFLAGS_DISALLOW_INSTANTIATION,
                       Py_TPFLAGS_IMMUTABLETYPE)


NOTHING = object()

class FieldsTestBase:
    # Structure/Union classes must get 'finalized' sooner or
    # later, when one of these things happen:
    #
    # 1. _fields_ is set.
    # 2. An instance is created.
    # 3. The type is used as field of another Structure/Union.
    # 4. The type is subclassed
    #
    # When they are finalized, assigning _fields_ is no longer allowed.

    def assert_final_fields(self, cls, expected=NOTHING):
        self.assertRaises(AttributeError, setattr, cls, "_fields_", [])
        self.assertEqual(getattr(cls, "_fields_", NOTHING), expected)

    def test_1_A(self):
        class X(self.cls):
            pass
        self.assertEqual(sizeof(X), 0) # not finalized
        X._fields_ = [] # finalized
        self.assert_final_fields(X, expected=[])

    def test_1_B(self):
        class X(self.cls):
            _fields_ = [] # finalized
        self.assert_final_fields(X, expected=[])

    def test_2(self):
        class X(self.cls):
            pass
        X()
        self.assert_final_fields(X)

    def test_3(self):
        class X(self.cls):
            pass
        class Y(self.cls):
            _fields_ = [("x", X)] # finalizes X
        self.assert_final_fields(X)

    def test_4(self):
        class X(self.cls):
            pass
        class Y(X):
            pass
        self.assert_final_fields(X)
        Y._fields_ = []
        self.assert_final_fields(X)

    def test_5(self):
        class X(self.cls):
            _fields_ = (("char", c_char * 5),)

        x = X(b'#' * 5)
        x.char = b'a\0b\0'
        self.assertEqual(bytes(x), b'a\x00###')

    def test_6(self):
        self.assertRaises(TypeError, CField)

    def test_gh99275(self):
        class BrokenStructure(self.cls):
            def __init_subclass__(cls, **kwargs):
                cls._fields_ = []  # This line will fail, `stginfo` is not ready

        with self.assertRaisesRegex(TypeError,
                                    'ctypes state is not initialized'):
            class Subclass(BrokenStructure): ...

    # __set__ and __get__ should raise a TypeError in case their self
    # argument is not a ctype instance.
    def test___set__(self):
        class MyCStruct(self.cls):
            _fields_ = (("field", c_int),)
        self.assertRaises(TypeError,
                          MyCStruct.field.__set__, 'wrong type self', 42)

    def test___get__(self):
        class MyCStruct(self.cls):
            _fields_ = (("field", c_int),)
        self.assertRaises(TypeError,
                          MyCStruct.field.__get__, 'wrong type self', 42)

class StructFieldsTestCase(unittest.TestCase, FieldsTestBase):
    cls = Structure

    def test_cfield_type_flags(self):
        self.assertTrue(CField.__flags__ & Py_TPFLAGS_IMMUTABLETYPE)

    def test_cfield_inheritance_hierarchy(self):
        self.assertEqual(CField.mro(), [CField, object])

class UnionFieldsTestCase(unittest.TestCase, FieldsTestBase):
    cls = Union


if __name__ == "__main__":
    unittest.main()
