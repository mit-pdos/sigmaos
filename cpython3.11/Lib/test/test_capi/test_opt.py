import contextlib
import sys
import textwrap
import unittest
import gc
import os

import _opcode

from test.support import (script_helper, requires_specialization,
                          import_helper, Py_GIL_DISABLED)

_testinternalcapi = import_helper.import_module("_testinternalcapi")

from _testinternalcapi import TIER2_THRESHOLD

@contextlib.contextmanager
def temporary_optimizer(opt):
    old_opt = _testinternalcapi.get_optimizer()
    _testinternalcapi.set_optimizer(opt)
    try:
        yield
    finally:
        _testinternalcapi.set_optimizer(old_opt)


@contextlib.contextmanager
def clear_executors(func):
    # Clear executors in func before and after running a block
    func.__code__ = func.__code__.replace()
    try:
        yield
    finally:
        func.__code__ = func.__code__.replace()


@requires_specialization
@unittest.skipIf(Py_GIL_DISABLED, "optimizer not yet supported in free-threaded builds")
@unittest.skipUnless(hasattr(_testinternalcapi, "get_optimizer"),
                     "Requires optimizer infrastructure")
class TestOptimizerAPI(unittest.TestCase):

    def test_new_counter_optimizer_dealloc(self):
        # See gh-108727
        def f():
            _testinternalcapi.new_counter_optimizer()

        f()

    def test_get_set_optimizer(self):
        old = _testinternalcapi.get_optimizer()
        opt = _testinternalcapi.new_counter_optimizer()
        try:
            _testinternalcapi.set_optimizer(opt)
            self.assertEqual(_testinternalcapi.get_optimizer(), opt)
            _testinternalcapi.set_optimizer(None)
            self.assertEqual(_testinternalcapi.get_optimizer(), None)
        finally:
            _testinternalcapi.set_optimizer(old)


    def test_counter_optimizer(self):
        # Generate a new function at each call
        ns = {}
        exec(textwrap.dedent("""
            def loop():
                for _ in range(1000):
                    pass
        """), ns, ns)
        loop = ns['loop']

        for repeat in range(5):
            opt = _testinternalcapi.new_counter_optimizer()
            with temporary_optimizer(opt):
                self.assertEqual(opt.get_count(), 0)
                with clear_executors(loop):
                    loop()
                # Subtract because optimizer doesn't kick in sooner
                self.assertEqual(opt.get_count(), 1000 - TIER2_THRESHOLD)

    def test_long_loop(self):
        "Check that we aren't confused by EXTENDED_ARG"

        # Generate a new function at each call
        ns = {}
        exec(textwrap.dedent("""
            def nop():
                pass

            def long_loop():
                for _ in range(20):
                    nop(); nop(); nop(); nop(); nop(); nop(); nop(); nop();
                    nop(); nop(); nop(); nop(); nop(); nop(); nop(); nop();
                    nop(); nop(); nop(); nop(); nop(); nop(); nop(); nop();
                    nop(); nop(); nop(); nop(); nop(); nop(); nop(); nop();
                    nop(); nop(); nop(); nop(); nop(); nop(); nop(); nop();
                    nop(); nop(); nop(); nop(); nop(); nop(); nop(); nop();
                    nop(); nop(); nop(); nop(); nop(); nop(); nop(); nop();
        """), ns, ns)
        long_loop = ns['long_loop']

        opt = _testinternalcapi.new_counter_optimizer()
        with temporary_optimizer(opt):
            self.assertEqual(opt.get_count(), 0)
            long_loop()
            self.assertEqual(opt.get_count(), 20 - TIER2_THRESHOLD)  # Need iterations to warm up

    def test_code_restore_for_ENTER_EXECUTOR(self):
        def testfunc(x):
            i = 0
            while i < x:
                i += 1

        opt = _testinternalcapi.new_counter_optimizer()
        with temporary_optimizer(opt):
            testfunc(1000)
            code, replace_code  = testfunc.__code__, testfunc.__code__.replace()
            self.assertEqual(code, replace_code)
            self.assertEqual(hash(code), hash(replace_code))


def get_first_executor(func):
    code = func.__code__
    co_code = code.co_code
    for i in range(0, len(co_code), 2):
        try:
            return _opcode.get_executor(code, i)
        except ValueError:
            pass
    return None


def iter_opnames(ex):
    for item in ex:
        yield item[0]


def get_opnames(ex):
    return list(iter_opnames(ex))


@requires_specialization
@unittest.skipIf(Py_GIL_DISABLED, "optimizer not yet supported in free-threaded builds")
@unittest.skipUnless(hasattr(_testinternalcapi, "get_optimizer"),
                     "Requires optimizer infrastructure")
class TestExecutorInvalidation(unittest.TestCase):

    def setUp(self):
        self.old = _testinternalcapi.get_optimizer()
        self.opt = _testinternalcapi.new_counter_optimizer()
        _testinternalcapi.set_optimizer(self.opt)

    def tearDown(self):
        _testinternalcapi.set_optimizer(self.old)

    def test_invalidate_object(self):
        # Generate a new set of functions at each call
        ns = {}
        func_src = "\n".join(
            f"""
            def f{n}():
                for _ in range(1000):
                    pass
            """ for n in range(5)
        )
        exec(textwrap.dedent(func_src), ns, ns)
        funcs = [ ns[f'f{n}'] for n in range(5)]
        objects = [object() for _ in range(5)]

        for f in funcs:
            f()
        executors = [get_first_executor(f) for f in funcs]
        # Set things up so each executor depends on the objects
        # with an equal or lower index.
        for i, exe in enumerate(executors):
            self.assertTrue(exe.is_valid())
            for obj in objects[:i+1]:
                _testinternalcapi.add_executor_dependency(exe, obj)
            self.assertTrue(exe.is_valid())
        # Assert that the correct executors are invalidated
        # and check that nothing crashes when we invalidate
        # an executor multiple times.
        for i in (4,3,2,1,0):
            _testinternalcapi.invalidate_executors(objects[i])
            for exe in executors[i:]:
                self.assertFalse(exe.is_valid())
            for exe in executors[:i]:
                self.assertTrue(exe.is_valid())

    def test_uop_optimizer_invalidation(self):
        # Generate a new function at each call
        ns = {}
        exec(textwrap.dedent("""
            def f():
                for i in range(1000):
                    pass
        """), ns, ns)
        f = ns['f']
        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            f()
        exe = get_first_executor(f)
        self.assertIsNotNone(exe)
        self.assertTrue(exe.is_valid())
        _testinternalcapi.invalidate_executors(f.__code__)
        self.assertFalse(exe.is_valid())

    def test_sys__clear_internal_caches(self):
        def f():
            for _ in range(1000):
                pass
        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            f()
        exe = get_first_executor(f)
        self.assertIsNotNone(exe)
        self.assertTrue(exe.is_valid())
        sys._clear_internal_caches()
        self.assertFalse(exe.is_valid())
        exe = get_first_executor(f)
        self.assertIsNone(exe)


@requires_specialization
@unittest.skipIf(Py_GIL_DISABLED, "optimizer not yet supported in free-threaded builds")
@unittest.skipUnless(hasattr(_testinternalcapi, "get_optimizer"),
                     "Requires optimizer infrastructure")
@unittest.skipIf(os.getenv("PYTHON_UOPS_OPTIMIZE") == "0", "Needs uop optimizer to run.")
class TestUops(unittest.TestCase):

    def test_basic_loop(self):
        def testfunc(x):
            i = 0
            while i < x:
                i += 1

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            testfunc(1000)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_JUMP_TO_TOP", uops)
        self.assertIn("_LOAD_FAST_0", uops)

    def test_extended_arg(self):
        "Check EXTENDED_ARG handling in superblock creation"
        ns = {}
        exec(textwrap.dedent("""
            def many_vars():
                # 260 vars, so z9 should have index 259
                a0 = a1 = a2 = a3 = a4 = a5 = a6 = a7 = a8 = a9 = 42
                b0 = b1 = b2 = b3 = b4 = b5 = b6 = b7 = b8 = b9 = 42
                c0 = c1 = c2 = c3 = c4 = c5 = c6 = c7 = c8 = c9 = 42
                d0 = d1 = d2 = d3 = d4 = d5 = d6 = d7 = d8 = d9 = 42
                e0 = e1 = e2 = e3 = e4 = e5 = e6 = e7 = e8 = e9 = 42
                f0 = f1 = f2 = f3 = f4 = f5 = f6 = f7 = f8 = f9 = 42
                g0 = g1 = g2 = g3 = g4 = g5 = g6 = g7 = g8 = g9 = 42
                h0 = h1 = h2 = h3 = h4 = h5 = h6 = h7 = h8 = h9 = 42
                i0 = i1 = i2 = i3 = i4 = i5 = i6 = i7 = i8 = i9 = 42
                j0 = j1 = j2 = j3 = j4 = j5 = j6 = j7 = j8 = j9 = 42
                k0 = k1 = k2 = k3 = k4 = k5 = k6 = k7 = k8 = k9 = 42
                l0 = l1 = l2 = l3 = l4 = l5 = l6 = l7 = l8 = l9 = 42
                m0 = m1 = m2 = m3 = m4 = m5 = m6 = m7 = m8 = m9 = 42
                n0 = n1 = n2 = n3 = n4 = n5 = n6 = n7 = n8 = n9 = 42
                o0 = o1 = o2 = o3 = o4 = o5 = o6 = o7 = o8 = o9 = 42
                p0 = p1 = p2 = p3 = p4 = p5 = p6 = p7 = p8 = p9 = 42
                q0 = q1 = q2 = q3 = q4 = q5 = q6 = q7 = q8 = q9 = 42
                r0 = r1 = r2 = r3 = r4 = r5 = r6 = r7 = r8 = r9 = 42
                s0 = s1 = s2 = s3 = s4 = s5 = s6 = s7 = s8 = s9 = 42
                t0 = t1 = t2 = t3 = t4 = t5 = t6 = t7 = t8 = t9 = 42
                u0 = u1 = u2 = u3 = u4 = u5 = u6 = u7 = u8 = u9 = 42
                v0 = v1 = v2 = v3 = v4 = v5 = v6 = v7 = v8 = v9 = 42
                w0 = w1 = w2 = w3 = w4 = w5 = w6 = w7 = w8 = w9 = 42
                x0 = x1 = x2 = x3 = x4 = x5 = x6 = x7 = x8 = x9 = 42
                y0 = y1 = y2 = y3 = y4 = y5 = y6 = y7 = y8 = y9 = 42
                z0 = z1 = z2 = z3 = z4 = z5 = z6 = z7 = z8 = z9 = 42
                while z9 > 0:
                    z9 = z9 - 1
                    +z9
        """), ns, ns)
        many_vars = ns["many_vars"]

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            ex = get_first_executor(many_vars)
            self.assertIsNone(ex)
            many_vars()

        ex = get_first_executor(many_vars)
        self.assertIsNotNone(ex)
        self.assertTrue(any((opcode, oparg, operand) == ("_LOAD_FAST", 259, 0)
                            for opcode, oparg, _, operand in list(ex)))

    def test_unspecialized_unpack(self):
        # An example of an unspecialized opcode
        def testfunc(x):
            i = 0
            while i < x:
                i += 1
                a, b = {1: 2, 3: 3}
            assert a == 1 and b == 3
            i = 0
            while i < x:
                i += 1

        opt = _testinternalcapi.new_uop_optimizer()

        with temporary_optimizer(opt):
            testfunc(20)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_UNPACK_SEQUENCE", uops)

    def test_pop_jump_if_false(self):
        def testfunc(n):
            i = 0
            while i < n:
                i += 1

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            testfunc(20)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_GUARD_IS_TRUE_POP", uops)

    def test_pop_jump_if_none(self):
        def testfunc(a):
            for x in a:
                if x is None:
                    x = 0

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            testfunc(range(20))

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertNotIn("_GUARD_IS_NONE_POP", uops)
        self.assertNotIn("_GUARD_IS_NOT_NONE_POP", uops)

    def test_pop_jump_if_not_none(self):
        def testfunc(a):
            for x in a:
                x = None
                if x is not None:
                    x = 0

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            testfunc(range(20))

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertNotIn("_GUARD_IS_NONE_POP", uops)
        self.assertNotIn("_GUARD_IS_NOT_NONE_POP", uops)

    def test_pop_jump_if_true(self):
        def testfunc(n):
            i = 0
            while not i >= n:
                i += 1

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            testfunc(20)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_GUARD_IS_FALSE_POP", uops)

    def test_jump_backward(self):
        def testfunc(n):
            i = 0
            while i < n:
                i += 1

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            testfunc(20)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_JUMP_TO_TOP", uops)

    def test_jump_forward(self):
        def testfunc(n):
            a = 0
            while a < n:
                if a < 0:
                    a = -a
                else:
                    a = +a
                a += 1
            return a

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            testfunc(20)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        # Since there is no JUMP_FORWARD instruction,
        # look for indirect evidence: the += operator
        self.assertIn("_BINARY_OP_ADD_INT", uops)

    def test_for_iter_range(self):
        def testfunc(n):
            total = 0
            for i in range(n):
                total += i
            return total

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            total = testfunc(20)
            self.assertEqual(total, 190)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        # for i, (opname, oparg) in enumerate(ex):
        #     print(f"{i:4d}: {opname:<20s} {oparg:3d}")
        uops = get_opnames(ex)
        self.assertIn("_GUARD_NOT_EXHAUSTED_RANGE", uops)
        # Verification that the jump goes past END_FOR
        # is done by manual inspection of the output

    def test_for_iter_list(self):
        def testfunc(a):
            total = 0
            for i in a:
                total += i
            return total

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            a = list(range(20))
            total = testfunc(a)
            self.assertEqual(total, 190)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        # for i, (opname, oparg) in enumerate(ex):
        #     print(f"{i:4d}: {opname:<20s} {oparg:3d}")
        uops = get_opnames(ex)
        self.assertIn("_GUARD_NOT_EXHAUSTED_LIST", uops)
        # Verification that the jump goes past END_FOR
        # is done by manual inspection of the output

    def test_for_iter_tuple(self):
        def testfunc(a):
            total = 0
            for i in a:
                total += i
            return total

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            a = tuple(range(20))
            total = testfunc(a)
            self.assertEqual(total, 190)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        # for i, (opname, oparg) in enumerate(ex):
        #     print(f"{i:4d}: {opname:<20s} {oparg:3d}")
        uops = get_opnames(ex)
        self.assertIn("_GUARD_NOT_EXHAUSTED_TUPLE", uops)
        # Verification that the jump goes past END_FOR
        # is done by manual inspection of the output

    def test_list_edge_case(self):
        def testfunc(it):
            for x in it:
                pass

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            a = [1, 2, 3]
            it = iter(a)
            testfunc(it)
            a.append(4)
            with self.assertRaises(StopIteration):
                next(it)

    def test_call_py_exact_args(self):
        def testfunc(n):
            def dummy(x):
                return x+1
            for i in range(n):
                dummy(i)

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            testfunc(20)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_PUSH_FRAME", uops)
        self.assertIn("_BINARY_OP_ADD_INT", uops)

    def test_branch_taken(self):
        def testfunc(n):
            for i in range(n):
                if i < 0:
                    i = 0
                else:
                    i = 1

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            testfunc(20)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_GUARD_IS_FALSE_POP", uops)

    def test_for_iter_tier_two(self):
        class MyIter:
            def __init__(self, n):
                self.n = n
            def __iter__(self):
                return self
            def __next__(self):
                self.n -= 1
                if self.n < 0:
                    raise StopIteration
                return self.n

        def testfunc(n, m):
            x = 0
            for i in range(m):
                for j in MyIter(n):
                    x += 1000*i + j
            return x

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            x = testfunc(10, 10)

        self.assertEqual(x, sum(range(10)) * 10010)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_FOR_ITER_TIER_TWO", uops)

    def test_confidence_score(self):
        def testfunc(n):
            bits = 0
            for i in range(n):
                if i & 0x01:
                    bits += 1
                if i & 0x02:
                    bits += 1
                if i&0x04:
                    bits += 1
                if i&0x08:
                    bits += 1
                if i&0x10:
                    bits += 1
                if i&0x20:
                    bits += 1
            return bits

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            x = testfunc(20)

        self.assertEqual(x, 40)
        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        ops = list(iter_opnames(ex))
        #Since branch is 50/50 the trace could go either way.
        count = ops.count("_GUARD_IS_TRUE_POP") + ops.count("_GUARD_IS_FALSE_POP")
        self.assertLessEqual(count, 2)


@requires_specialization
@unittest.skipIf(Py_GIL_DISABLED, "optimizer not yet supported in free-threaded builds")
@unittest.skipUnless(hasattr(_testinternalcapi, "get_optimizer"),
                     "Requires optimizer infrastructure")
@unittest.skipIf(os.getenv("PYTHON_UOPS_OPTIMIZE") == "0", "Needs uop optimizer to run.")
class TestUopsOptimization(unittest.TestCase):

    def _run_with_optimizer(self, testfunc, arg):
        res = None
        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            res = testfunc(arg)

        ex = get_first_executor(testfunc)
        return res, ex


    def test_int_type_propagation(self):
        def testfunc(loops):
            num = 0
            for i in range(loops):
                x = num + num
                a = x + 1
                num += 1
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertIsNotNone(ex)
        self.assertEqual(res, 63)
        binop_count = [opname for opname in iter_opnames(ex) if opname == "_BINARY_OP_ADD_INT"]
        guard_both_int_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_INT"]
        self.assertGreaterEqual(len(binop_count), 3)
        self.assertLessEqual(len(guard_both_int_count), 1)

    def test_int_type_propagation_through_frame(self):
        def double(x):
            return x + x
        def testfunc(loops):
            num = 0
            for i in range(loops):
                x = num + num
                a = double(x)
                num += 1
            return a

        opt = _testinternalcapi.new_uop_optimizer()
        res = None
        with temporary_optimizer(opt):
            res = testfunc(32)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        self.assertEqual(res, 124)
        binop_count = [opname for opname in iter_opnames(ex) if opname == "_BINARY_OP_ADD_INT"]
        guard_both_int_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_INT"]
        self.assertGreaterEqual(len(binop_count), 3)
        self.assertLessEqual(len(guard_both_int_count), 1)

    def test_int_type_propagation_from_frame(self):
        def double(x):
            return x + x
        def testfunc(loops):
            num = 0
            for i in range(loops):
                a = double(num)
                x = a + a
                num += 1
            return x

        opt = _testinternalcapi.new_uop_optimizer()
        res = None
        with temporary_optimizer(opt):
            res = testfunc(32)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        self.assertEqual(res, 124)
        binop_count = [opname for opname in iter_opnames(ex) if opname == "_BINARY_OP_ADD_INT"]
        guard_both_int_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_INT"]
        self.assertGreaterEqual(len(binop_count), 3)
        self.assertLessEqual(len(guard_both_int_count), 1)

    def test_int_impure_region(self):
        def testfunc(loops):
            num = 0
            while num < loops:
                x = num + num
                y = 1
                x // 2
                a = x + y
                num += 1
            return a

        res, ex = self._run_with_optimizer(testfunc, 64)
        self.assertIsNotNone(ex)
        binop_count = [opname for opname in iter_opnames(ex) if opname == "_BINARY_OP_ADD_INT"]
        self.assertGreaterEqual(len(binop_count), 3)

    def test_call_py_exact_args(self):
        def testfunc(n):
            def dummy(x):
                return x+1
            for i in range(n):
                dummy(i)

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_PUSH_FRAME", uops)
        self.assertIn("_BINARY_OP_ADD_INT", uops)
        self.assertNotIn("_CHECK_PEP_523", uops)

    def test_int_type_propagate_through_range(self):
        def testfunc(n):

            for i in range(n):
                x = i + i
            return x

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 62)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertNotIn("_GUARD_BOTH_INT", uops)

    def test_int_value_numbering(self):
        def testfunc(n):

            y = 1
            for i in range(n):
                x = y
                z = x
                a = z
                b = a
                res = x + z + a + b
            return res

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 4)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_GUARD_BOTH_INT", uops)
        guard_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_INT"]
        self.assertEqual(len(guard_count), 1)

    def test_comprehension(self):
        def testfunc(n):
            for _ in range(n):
                return [i for i in range(n)]

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, list(range(32)))
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertNotIn("_BINARY_OP_ADD_INT", uops)

    def test_call_py_exact_args_disappearing(self):
        def dummy(x):
            return x+1

        def testfunc(n):
            for i in range(n):
                dummy(i)

        opt = _testinternalcapi.new_uop_optimizer()
        # Trigger specialization
        testfunc(8)
        with temporary_optimizer(opt):
            del dummy
            gc.collect()

            def dummy(x):
                return x + 2
            testfunc(32)

        ex = get_first_executor(testfunc)
        # Honestly as long as it doesn't crash it's fine.
        # Whether we get an executor or not is non-deterministic,
        # because it's decided by when the function is freed.
        # This test is a little implementation specific.

    def test_promote_globals_to_constants(self):

        result = script_helper.run_python_until_end('-c', textwrap.dedent("""
        import _testinternalcapi
        import opcode
        import _opcode

        def get_first_executor(func):
            code = func.__code__
            co_code = code.co_code
            for i in range(0, len(co_code), 2):
                try:
                    return _opcode.get_executor(code, i)
                except ValueError:
                    pass
            return None

        def get_opnames(ex):
            return {item[0] for item in ex}

        def testfunc(n):
            for i in range(n):
                x = range(i)
            return x

        opt = _testinternalcapi.new_uop_optimizer()
        _testinternalcapi.set_optimizer(opt)
        testfunc(64)

        ex = get_first_executor(testfunc)
        assert ex is not None
        uops = get_opnames(ex)
        assert "_LOAD_GLOBAL_BUILTINS" not in uops
        assert "_LOAD_CONST_INLINE_BORROW_WITH_NULL" in uops
        """))
        self.assertEqual(result[0].rc, 0, result)

    def test_float_add_constant_propagation(self):
        def testfunc(n):
            a = 1.0
            for _ in range(n):
                a = a + 0.25
                a = a + 0.25
                a = a + 0.25
                a = a + 0.25
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertAlmostEqual(res, 33.0)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        guard_both_float_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_FLOAT"]
        self.assertLessEqual(len(guard_both_float_count), 1)
        # TODO gh-115506: this assertion may change after propagating constants.
        # We'll also need to verify that propagation actually occurs.
        self.assertIn("_BINARY_OP_ADD_FLOAT", uops)

    def test_float_subtract_constant_propagation(self):
        def testfunc(n):
            a = 1.0
            for _ in range(n):
                a = a - 0.25
                a = a - 0.25
                a = a - 0.25
                a = a - 0.25
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertAlmostEqual(res, -31.0)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        guard_both_float_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_FLOAT"]
        self.assertLessEqual(len(guard_both_float_count), 1)
        # TODO gh-115506: this assertion may change after propagating constants.
        # We'll also need to verify that propagation actually occurs.
        self.assertIn("_BINARY_OP_SUBTRACT_FLOAT", uops)

    def test_float_multiply_constant_propagation(self):
        def testfunc(n):
            a = 1.0
            for _ in range(n):
                a = a * 1.0
                a = a * 1.0
                a = a * 1.0
                a = a * 1.0
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertAlmostEqual(res, 1.0)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        guard_both_float_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_FLOAT"]
        self.assertLessEqual(len(guard_both_float_count), 1)
        # TODO gh-115506: this assertion may change after propagating constants.
        # We'll also need to verify that propagation actually occurs.
        self.assertIn("_BINARY_OP_MULTIPLY_FLOAT", uops)

    def test_add_unicode_propagation(self):
        def testfunc(n):
            a = ""
            for _ in range(n):
                a + a
                a + a
                a + a
                a + a
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, "")
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        guard_both_unicode_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_UNICODE"]
        self.assertLessEqual(len(guard_both_unicode_count), 1)
        self.assertIn("_BINARY_OP_ADD_UNICODE", uops)

    def test_compare_op_type_propagation_float(self):
        def testfunc(n):
            a = 1.0
            for _ in range(n):
                x = a == a
                x = a == a
                x = a == a
                x = a == a
            return x

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertTrue(res)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        guard_both_float_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_FLOAT"]
        self.assertLessEqual(len(guard_both_float_count), 1)
        self.assertIn("_COMPARE_OP_FLOAT", uops)

    def test_compare_op_type_propagation_int(self):
        def testfunc(n):
            a = 1
            for _ in range(n):
                x = a == a
                x = a == a
                x = a == a
                x = a == a
            return x

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertTrue(res)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        guard_both_int_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_INT"]
        self.assertLessEqual(len(guard_both_int_count), 1)
        self.assertIn("_COMPARE_OP_INT", uops)

    def test_compare_op_type_propagation_int_partial(self):
        def testfunc(n):
            a = 1
            for _ in range(n):
                if a > 2:
                    x = 0
                if a < 2:
                    x = 1
            return x

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 1)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        guard_left_int_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_NOS_INT"]
        guard_both_int_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_INT"]
        self.assertLessEqual(len(guard_left_int_count), 1)
        self.assertEqual(len(guard_both_int_count), 0)
        self.assertIn("_COMPARE_OP_INT", uops)

    def test_compare_op_type_propagation_float_partial(self):
        def testfunc(n):
            a = 1.0
            for _ in range(n):
                if a > 2.0:
                    x = 0
                if a < 2.0:
                    x = 1
            return x

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 1)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        guard_left_float_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_NOS_FLOAT"]
        guard_both_float_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_FLOAT"]
        self.assertLessEqual(len(guard_left_float_count), 1)
        self.assertEqual(len(guard_both_float_count), 0)
        self.assertIn("_COMPARE_OP_FLOAT", uops)

    def test_compare_op_type_propagation_unicode(self):
        def testfunc(n):
            a = ""
            for _ in range(n):
                x = a == a
                x = a == a
                x = a == a
                x = a == a
            return x

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertTrue(res)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        guard_both_float_count = [opname for opname in iter_opnames(ex) if opname == "_GUARD_BOTH_UNICODE"]
        self.assertLessEqual(len(guard_both_float_count), 1)
        self.assertIn("_COMPARE_OP_STR", uops)

    def test_type_inconsistency(self):
        ns = {}
        src = textwrap.dedent("""
            def testfunc(n):
                for i in range(n):
                    x = _test_global + _test_global
        """)
        exec(src, ns, ns)
        testfunc = ns['testfunc']
        ns['_test_global'] = 0
        _, ex = self._run_with_optimizer(testfunc, TIER2_THRESHOLD)
        self.assertIsNone(ex)
        ns['_test_global'] = 1
        _, ex = self._run_with_optimizer(testfunc, TIER2_THRESHOLD)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertNotIn("_GUARD_BOTH_INT", uops)
        self.assertIn("_BINARY_OP_ADD_INT", uops)
        # Try again, but between the runs, set the global to a float.
        # This should result in no executor the second time.
        ns = {}
        exec(src, ns, ns)
        testfunc = ns['testfunc']
        ns['_test_global'] = 0
        _, ex = self._run_with_optimizer(testfunc, TIER2_THRESHOLD)
        self.assertIsNone(ex)
        ns['_test_global'] = 3.14
        _, ex = self._run_with_optimizer(testfunc, TIER2_THRESHOLD)
        self.assertIsNone(ex)

    def test_combine_stack_space_checks_sequential(self):
        def dummy12(x):
            return x - 1
        def dummy13(y):
            z = y + 2
            return y, z
        def testfunc(n):
            a = 0
            for _ in range(n):
                b = dummy12(7)
                c, d = dummy13(9)
                a += b + c + d
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 832)
        self.assertIsNotNone(ex)

        uops_and_operands = [(opcode, operand) for opcode, _, _, operand in ex]
        uop_names = [uop[0] for uop in uops_and_operands]
        self.assertEqual(uop_names.count("_PUSH_FRAME"), 2)
        self.assertEqual(uop_names.count("_RETURN_VALUE"), 2)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE"), 0)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE_OPERAND"), 1)
        # sequential calls: max(12, 13) == 13
        largest_stack = _testinternalcapi.get_co_framesize(dummy13.__code__)
        self.assertIn(("_CHECK_STACK_SPACE_OPERAND", largest_stack), uops_and_operands)

    def test_combine_stack_space_checks_nested(self):
        def dummy12(x):
            return x + 3
        def dummy15(y):
            z = dummy12(y)
            return y, z
        def testfunc(n):
            a = 0
            for _ in range(n):
                b, c = dummy15(2)
                a += b + c
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 224)
        self.assertIsNotNone(ex)

        uops_and_operands = [(opcode, operand) for opcode, _, _, operand in ex]
        uop_names = [uop[0] for uop in uops_and_operands]
        self.assertEqual(uop_names.count("_PUSH_FRAME"), 2)
        self.assertEqual(uop_names.count("_RETURN_VALUE"), 2)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE"), 0)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE_OPERAND"), 1)
        # nested calls: 15 + 12 == 27
        largest_stack = (
            _testinternalcapi.get_co_framesize(dummy15.__code__) +
            _testinternalcapi.get_co_framesize(dummy12.__code__)
        )
        self.assertIn(("_CHECK_STACK_SPACE_OPERAND", largest_stack), uops_and_operands)

    def test_combine_stack_space_checks_several_calls(self):
        def dummy12(x):
            return x + 3
        def dummy13(y):
            z = y + 2
            return y, z
        def dummy18(y):
            z = dummy12(y)
            x, w = dummy13(z)
            return z, x, w
        def testfunc(n):
            a = 0
            for _ in range(n):
                b = dummy12(5)
                c, d, e = dummy18(2)
                a += b + c + d + e
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 800)
        self.assertIsNotNone(ex)

        uops_and_operands = [(opcode, operand) for opcode, _, _, operand in ex]
        uop_names = [uop[0] for uop in uops_and_operands]
        self.assertEqual(uop_names.count("_PUSH_FRAME"), 4)
        self.assertEqual(uop_names.count("_RETURN_VALUE"), 4)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE"), 0)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE_OPERAND"), 1)
        # max(12, 18 + max(12, 13)) == 31
        largest_stack = (
            _testinternalcapi.get_co_framesize(dummy18.__code__) +
            _testinternalcapi.get_co_framesize(dummy13.__code__)
        )
        self.assertIn(("_CHECK_STACK_SPACE_OPERAND", largest_stack), uops_and_operands)

    def test_combine_stack_space_checks_several_calls_different_order(self):
        # same as `several_calls` but with top-level calls reversed
        def dummy12(x):
            return x + 3
        def dummy13(y):
            z = y + 2
            return y, z
        def dummy18(y):
            z = dummy12(y)
            x, w = dummy13(z)
            return z, x, w
        def testfunc(n):
            a = 0
            for _ in range(n):
                c, d, e = dummy18(2)
                b = dummy12(5)
                a += b + c + d + e
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 800)
        self.assertIsNotNone(ex)

        uops_and_operands = [(opcode, operand) for opcode, _, _, operand in ex]
        uop_names = [uop[0] for uop in uops_and_operands]
        self.assertEqual(uop_names.count("_PUSH_FRAME"), 4)
        self.assertEqual(uop_names.count("_RETURN_VALUE"), 4)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE"), 0)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE_OPERAND"), 1)
        # max(18 + max(12, 13), 12) == 31
        largest_stack = (
            _testinternalcapi.get_co_framesize(dummy18.__code__) +
            _testinternalcapi.get_co_framesize(dummy13.__code__)
        )
        self.assertIn(("_CHECK_STACK_SPACE_OPERAND", largest_stack), uops_and_operands)

    def test_combine_stack_space_complex(self):
        def dummy0(x):
            return x
        def dummy1(x):
            return dummy0(x)
        def dummy2(x):
            return dummy1(x)
        def dummy3(x):
            return dummy0(x)
        def dummy4(x):
            y = dummy0(x)
            return dummy3(y)
        def dummy5(x):
            return dummy2(x)
        def dummy6(x):
            y = dummy5(x)
            z = dummy0(y)
            return dummy4(z)
        def testfunc(n):
            a = 0;
            for _ in range(32):
                b = dummy5(1)
                c = dummy0(1)
                d = dummy6(1)
                a += b + c + d
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 96)
        self.assertIsNotNone(ex)

        uops_and_operands = [(opcode, operand) for opcode, _, _, operand in ex]
        uop_names = [uop[0] for uop in uops_and_operands]
        self.assertEqual(uop_names.count("_PUSH_FRAME"), 15)
        self.assertEqual(uop_names.count("_RETURN_VALUE"), 15)

        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE"), 0)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE_OPERAND"), 1)
        largest_stack = (
            _testinternalcapi.get_co_framesize(dummy6.__code__) +
            _testinternalcapi.get_co_framesize(dummy5.__code__) +
            _testinternalcapi.get_co_framesize(dummy2.__code__) +
            _testinternalcapi.get_co_framesize(dummy1.__code__) +
            _testinternalcapi.get_co_framesize(dummy0.__code__)
        )
        self.assertIn(
            ("_CHECK_STACK_SPACE_OPERAND", largest_stack), uops_and_operands
        )

    def test_combine_stack_space_checks_large_framesize(self):
        # Create a function with a large framesize. This ensures _CHECK_STACK_SPACE is
        # actually doing its job. Note that the resulting trace hits
        # UOP_MAX_TRACE_LENGTH, but since all _CHECK_STACK_SPACEs happen early, this
        # test is still meaningful.
        repetitions = 10000
        ns = {}
        header = """
            def dummy_large(a0):
        """
        body = "".join([f"""
                a{n+1} = a{n} + 1
        """ for n in range(repetitions)])
        return_ = f"""
                return a{repetitions-1}
        """
        exec(textwrap.dedent(header + body + return_), ns, ns)
        dummy_large = ns['dummy_large']

        # this is something like:
        #
        # def dummy_large(a0):
        #     a1 = a0 + 1
        #     a2 = a1 + 1
        #     ....
        #     a9999 = a9998 + 1
        #     return a9999

        def dummy15(z):
            y = dummy_large(z)
            return y + 3

        def testfunc(n):
            b = 0
            for _ in range(n):
                b += dummy15(7)
            return b

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 32 * (repetitions + 9))
        self.assertIsNotNone(ex)

        uops_and_operands = [(opcode, operand) for opcode, _, _, operand in ex]
        uop_names = [uop[0] for uop in uops_and_operands]
        self.assertEqual(uop_names.count("_PUSH_FRAME"), 2)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE_OPERAND"), 1)

        # this hits a different case during trace projection in refcount test runs only,
        # so we need to account for both possibilities
        self.assertIn(uop_names.count("_CHECK_STACK_SPACE"), [0, 1])
        if uop_names.count("_CHECK_STACK_SPACE") == 0:
            largest_stack = (
                _testinternalcapi.get_co_framesize(dummy15.__code__) +
                _testinternalcapi.get_co_framesize(dummy_large.__code__)
            )
        else:
            largest_stack = _testinternalcapi.get_co_framesize(dummy15.__code__)
        self.assertIn(
            ("_CHECK_STACK_SPACE_OPERAND", largest_stack), uops_and_operands
        )

    def test_combine_stack_space_checks_recursion(self):
        def dummy15(x):
            while x > 0:
                return dummy15(x - 1)
            return 42
        def testfunc(n):
            a = 0
            for _ in range(n):
                a += dummy15(n)
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 42 * 32)
        self.assertIsNotNone(ex)

        uops_and_operands = [(opcode, operand) for opcode, _, _, operand in ex]
        uop_names = [uop[0] for uop in uops_and_operands]
        self.assertEqual(uop_names.count("_PUSH_FRAME"), 2)
        self.assertEqual(uop_names.count("_RETURN_VALUE"), 0)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE"), 1)
        self.assertEqual(uop_names.count("_CHECK_STACK_SPACE_OPERAND"), 1)
        largest_stack = _testinternalcapi.get_co_framesize(dummy15.__code__)
        self.assertIn(("_CHECK_STACK_SPACE_OPERAND", largest_stack), uops_and_operands)

    def test_many_nested(self):
        # overflow the trace_stack
        def dummy_a(x):
            return x
        def dummy_b(x):
            return dummy_a(x)
        def dummy_c(x):
            return dummy_b(x)
        def dummy_d(x):
            return dummy_c(x)
        def dummy_e(x):
            return dummy_d(x)
        def dummy_f(x):
            return dummy_e(x)
        def dummy_g(x):
            return dummy_f(x)
        def dummy_h(x):
            return dummy_g(x)
        def testfunc(n):
            a = 0
            for _ in range(n):
                a += dummy_h(n)
            return a

        res, ex = self._run_with_optimizer(testfunc, 32)
        self.assertEqual(res, 32 * 32)
        self.assertIsNone(ex)

    def test_return_generator(self):
        def gen():
            yield None
        def testfunc(n):
            for i in range(n):
                gen()
            return i
        res, ex = self._run_with_optimizer(testfunc, 20)
        self.assertEqual(res, 19)
        self.assertIsNotNone(ex)
        self.assertIn("_RETURN_GENERATOR", get_opnames(ex))

    def test_for_iter_gen(self):
        def gen(n):
            for i in range(n):
                yield i
        def testfunc(n):
            g = gen(n)
            s = 0
            for i in g:
                s += i
            return s
        res, ex = self._run_with_optimizer(testfunc, 20)
        self.assertEqual(res, 190)
        self.assertIsNotNone(ex)
        self.assertIn("_FOR_ITER_GEN_FRAME", get_opnames(ex))

    def test_modified_local_is_seen_by_optimized_code(self):
        l = sys._getframe().f_locals
        a = 1
        s = 0
        for j in range(1 << 10):
            a + a
            l["xa"[j >> 9]] = 1.0
            s += a
        self.assertIs(type(a), float)
        self.assertIs(type(s), float)
        self.assertEqual(s, 1024.0)

    def test_guard_type_version_removed(self):
        def thing(a):
            x = 0
            for _ in range(100):
                x += a.attr
                x += a.attr
            return x

        class Foo:
            attr = 1

        res, ex = self._run_with_optimizer(thing, Foo())
        opnames = list(iter_opnames(ex))
        self.assertIsNotNone(ex)
        self.assertEqual(res, 200)
        guard_type_version_count = opnames.count("_GUARD_TYPE_VERSION")
        self.assertEqual(guard_type_version_count, 1)

    def test_guard_type_version_removed_inlined(self):
        """
        Verify that the guard type version if we have an inlined function
        """

        def fn():
            pass

        def thing(a):
            x = 0
            for _ in range(100):
                x += a.attr
                fn()
                x += a.attr
            return x

        class Foo:
            attr = 1

        res, ex = self._run_with_optimizer(thing, Foo())
        opnames = list(iter_opnames(ex))
        self.assertIsNotNone(ex)
        self.assertEqual(res, 200)
        guard_type_version_count = opnames.count("_GUARD_TYPE_VERSION")
        self.assertEqual(guard_type_version_count, 1)

    def test_guard_type_version_not_removed(self):
        """
        Verify that the guard type version is not removed if we modify the class
        """

        def thing(a):
            x = 0
            for i in range(100):
                x += a.attr
                # for the first 90 iterations we set the attribute on this dummy function which shouldn't
                # trigger the type watcher
                # then after 90  it should trigger it and stop optimizing
                # Note that the code needs to be in this weird form so it's optimized inline without any control flow
                setattr((Foo, Bar)[i < 90], "attr", 2)
                x += a.attr
            return x

        class Foo:
            attr = 1

        class Bar:
            pass

        res, ex = self._run_with_optimizer(thing, Foo())
        opnames = list(iter_opnames(ex))

        self.assertIsNotNone(ex)
        self.assertEqual(res, 219)
        guard_type_version_count = opnames.count("_GUARD_TYPE_VERSION")
        self.assertEqual(guard_type_version_count, 2)


    @unittest.expectedFailure
    def test_guard_type_version_not_removed_escaping(self):
        """
        Verify that the guard type version is not removed if have an escaping function
        """

        def thing(a):
            x = 0
            for i in range(100):
                x += a.attr
                # eval should be escaping and so should cause optimization to stop and preserve both type versions
                eval("None")
                x += a.attr
            return x

        class Foo:
            attr = 1
        res, ex = self._run_with_optimizer(thing, Foo())
        opnames = list(iter_opnames(ex))
        self.assertIsNotNone(ex)
        self.assertEqual(res, 200)
        guard_type_version_count = opnames.count("_GUARD_TYPE_VERSION")
        # Note: This will actually be 1 for noe
        # https://github.com/python/cpython/pull/119365#discussion_r1626220129
        self.assertEqual(guard_type_version_count, 2)


    def test_guard_type_version_executor_invalidated(self):
        """
        Verify that the executor is invalided on a type change.
        """

        def thing(a):
            x = 0
            for i in range(100):
                x += a.attr
                x += a.attr
            return x

        class Foo:
            attr = 1

        res, ex = self._run_with_optimizer(thing, Foo())
        self.assertEqual(res, 200)
        self.assertIsNotNone(ex)
        self.assertEqual(list(iter_opnames(ex)).count("_GUARD_TYPE_VERSION"), 1)
        self.assertTrue(ex.is_valid())
        Foo.attr = 0
        self.assertFalse(ex.is_valid())

    def test_type_version_doesnt_segfault(self):
        """
        Tests that setting a type version doesn't cause a segfault when later looking at the stack.
        """

        # Minimized from mdp.py benchmark

        class A:
            def __init__(self):
                self.attr = {}

            def method(self, arg):
                self.attr[arg] = None

        def fn(a):
            for _ in range(100):
                (_ for _ in [])
                (_ for _ in [a.method(None)])

        fn(A())

    def test_func_guards_removed_or_reduced(self):
        def testfunc(n):
            for i in range(n):
                # Only works on functions promoted to constants
                global_identity(i)

        opt = _testinternalcapi.new_uop_optimizer()
        with temporary_optimizer(opt):
            testfunc(20)

        ex = get_first_executor(testfunc)
        self.assertIsNotNone(ex)
        uops = get_opnames(ex)
        self.assertIn("_PUSH_FRAME", uops)
        # Strength reduced version
        self.assertIn("_CHECK_FUNCTION_VERSION_INLINE", uops)
        self.assertNotIn("_CHECK_FUNCTION_VERSION", uops)
        # Removed guard
        self.assertNotIn("_CHECK_FUNCTION_EXACT_ARGS", uops)

    def test_jit_error_pops(self):
        """
        Tests that the correct number of pops are inserted into the
        exit stub
        """
        items = 17 * [None] + [[]]
        with self.assertRaises(TypeError):
            {item for item in items}


def global_identity(x):
    return x

if __name__ == "__main__":
    unittest.main()
