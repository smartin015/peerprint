import unittest
import logging
import tempfile
from pathlib import Path
from .filesharing import pack_job, unpack_job, packed_name, Fileshare

class TestPackJob(unittest.TestCase):
    def setUp(self):
        self.tempdir = tempfile.TemporaryDirectory()
        p = Path(self.tempdir.name)
        self.pathnames = ['a.gcode', 'b.gcode', 'c.gcode']
        self.paths = dict([(n, p / n) for n in self.pathnames])
        for path in self.paths.values():
            path.touch()
        self.m = dict(sets=[dict(path=n) for n in self.pathnames])

    def tearDown(self):
        self.tempdir.cleanup()

    def test_pack_job_with_files(self):
        with tempfile.NamedTemporaryFile(suffix=".zip") as outpath:
            pack_job(self.m, self.paths, outpath.name)
            with tempfile.TemporaryDirectory() as td:
                result = unpack_job(outpath.name, td)
                result[0].pop("version")
                self.assertEqual(result[0], self.m)
                self.assertEqual([Path(p).name for p in result[1]], list(self.paths.keys()))

    def test_pack_job_hash_matching(self):
        with tempfile.NamedTemporaryFile(suffix=".zip") as tf1:
            with tempfile.NamedTemporaryFile(suffix=".zip") as tf2:
                h1 = pack_job(self.m, self.paths, tf1.name)
                h2 = pack_job(self.m, self.paths, tf2.name)
                self.assertEqual(h1, h2)

    def test_pack_job_hash_not_matching(self):
        with tempfile.NamedTemporaryFile(suffix=".zip") as tf1:
            with tempfile.NamedTemporaryFile(suffix=".zip") as tf2:
                h1 = pack_job(self.m, self.paths, tf1.name)
                self.m['sets'].pop(0)
                h2 = pack_job(self.m, dict(list(self.paths.items())[1:]), tf2.name)
                self.assertNotEqual(h1, h2)

    def test_throws_on_missing_file(self):
        self.m['sets'].append(dict(path="notexisting.gcode"))
        with tempfile.NamedTemporaryFile(suffix=".zip") as outpath:
            with self.assertRaises(ValueError):
                pack_job(self.m, self.paths, outpath.name)

class TestPackedName(unittest.TestCase):
    def testPacking(self):
        ts = 123
        for tc, want in [
            ("", "cpq_untitled_123.gjob"),
            ("hi", "cpq_hi_123.gjob"),
            ("!!**$$..", "cpq__123.gjob"),
            ("space is cool", "cpq_space_is_cool_123.gjob"),
                ]:
            self.assertEqual(packed_name(tc, ts), want)

class TestFileshare(unittest.TestCase):
    def setUp(self):
        NUMFS = 2
        self.td = [tempfile.TemporaryDirectory() for i in range(NUMFS)]
        self.fs = [Fileshare("localhost:0", self.td[i].name, logging.getLogger(f"TestFileshare{i}")) for i in range(NUMFS)]
        for fs in self.fs:
            fs.connect()
        self.addr = [f"localhost:{fs.httpd.socket.getsockname()[1]}" for fs in self.fs]

    def tearDown(self):
        for td in self.td:
            td.cleanup()

    def testPostReceive(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / 'a.gcode'
            DATA = "hello"
            with open(p, 'w') as f:
                f.write(DATA)
            m = dict(sets=[dict(path='a.gcode')])
            HASH = self.fs[0].post(m, {'a.gcode': p})
            dest = self.fs[1].fetch(self.addr[0], HASH, unpack=True)
            self.assertEqual(Path(dest).is_dir(), True)
            self.assertEqual((Path(dest) / 'a.gcode').exists(), True)
            with open(Path(dest) / 'a.gcode', 'r') as f:
                self.assertEqual(f.read(), DATA)

