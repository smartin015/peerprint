import unittest
from unittest.mock import MagicMock
from .sync_objects import CPOrderedReplDict

# Implement CPOrderedReplDict's abstract _item_changed function
# and also override replication on mutation methods
class TestReplDict(CPOrderedReplDict):
    def __init__(self, cb):
        super().__init__(cb)

    def _item_changed(self, prev, nxt):
        return prev != nxt

    def __setitem__(self, k, v, **kwargs):
        return self._setitem_impl(k,v)

    def pop(self, k,d=None):
        return self._pop_impl(k,d)

    def mv(self, key, after):
        return self._mv_impl(key, after)

class TestCPOrderedReplDictEmpty(unittest.TestCase):
    def setUp(self):
        self.m = MagicMock()
        self.d = TestReplDict(self.m)

    def testAddItemsInOrder(self):
        for a in ['d','c']:
            self.d[a] = a

        self.assertEqual(list(self.d.ordered_items()), [
            ('d', 'd'),
            ('c', 'c'),
        ])

    def testPopEmpty(self):
        self.assertEqual(self.d.pop('a'), None)

    def testMoveSingleItemInQueue(self):
        self.d['1'] = '1'
        self.d.mv('1', None)
        self.assertEqual(list(self.d.ordered_items()), [('1', '1')])

class TestCPOrderedReplDictWithItems(unittest.TestCase):
    def setUp(self):
        self.m = MagicMock()
        self.d = TestReplDict(self.m)
        for a in ['1','2','3','4']:
            self.d[a] = a

    def _orderedEquals(self, result):
        return self.assertEqual(
            [i[0] for i in list(self.d.ordered_items())],
            result)

    def testPopMiddle(self):
        self.d.pop('2')
        self._orderedEquals(['1','3','4'])

    def testPopLast(self):
        self.d.pop('4')
        self._orderedEquals(['1','2','3'])

    def testPopFirst(self):
        self.d.pop('1')
        self._orderedEquals(['2','3','4'])

    def testMoveExchangeMiddle(self):
        self.d.mv('2', '3')
        self._orderedEquals(['1','3','2','4'])

        # Reverse direction
        self.d.mv('2', '1')
        self._orderedEquals(['1','2','3','4'])

    def testMoveLastToFirst(self):
        self.d.mv('4', None)
        self._orderedEquals(['4','1','2','3'])

    def testMoveFirstToLast(self):
        self.d.mv('1', '4')
        self._orderedEquals(['2','3','4','1'])

    def testMoveLastToMiddle(self):
        self.d.mv('4', '2')
        self._orderedEquals(['1','2','4','3'])

    def testMoveBadAfterCausesNoChange(self):
        with self.assertRaises(KeyError):
            self.d.mv('4', '80')
        self._orderedEquals(['1','2','3','4'])

    def testMoveBadKeyCausesNoChange(self):
        with self.assertRaises(KeyError):
            self.d.mv('80', '2')
        self._orderedEquals(['1','2','3','4'])
