
function AppViewModel(hash) {
  let self = this;
  self.serverSummary = ko.observable();
  self.storageSummary = ko.observable();
  self.instances = ko.observableArray([]);
  self.credentials = ko.observableArray([]);
  self.connections = ko.observableArray([]);
  self.events = ko.observableArray([]);
  self.peerTracking = ko.observableArray([]);
  self.peerStatuses = ko.observableArray([]);
  self.clientStatuses = ko.computed(function() {
    let result = [];
    for (let p of self.peerStatuses()) {
      for (let c of p.clients) {
        result.push({...c, peer: p.name});
      }
    }
    return result;
  });
  self.lobby = ko.observableArray([]);
  self.lobbyStats = ko.observable({});
  self.runtil = ko.observable("n/a");
  self.registry = ko.observableArray([]);
	self.toastData = ko.observable({title: "", body: ""});
	self.showToast = function(title, body) {
		self.toastData({title, body});
	};
  self.timeline = new PeerTimeline('peerTimeline', 'Census Timeline', 'Time', 'Peer Count');
  self.geo = new PeerGeoMap('peerGeo', 'Client Locations');
  self.geoUpdater = ko.computed(function() {
    let pts = [];
    for (let p of self.peerStatuses()) {
      for (let c of p.clients) {
        if (!c.location) {
          continue;
        }
        pts.push([c.location.latitude, c.location.longitude, `${p.name} ${c.name} (${c.status})`]);
      }
    }
    self.geo.update(pts);
    return pts;
  });

  setInterval(() => {
    const ls = self.lobbyStats();
    if (ls && ls.local && ls.local.Deadline !== undefined) {
      const ts = new Date(ls.local.Deadline);
      const dt = ts - (new Date());
      if (dt > 0) {
        self.runtil(`${Math.floor(dt/1000)}s`);
      } else {
        self.runtil("n/a");
      }
    }
  }, 500);

	self.shownTab = ko.observable(null);
	self.gotoTab = function(v, e) {
		const tabel = document.querySelector('#' + v + '-tab');
		if (tabel) {
			const tab = new bootstrap.Tab(tabel);
			tab.show();
			self.shownTab(v);
		}
    if (e !== undefined) {
      e.preventDefault();
    }
	}
	self.gotoTab(hash[1] || 'server');
  self.selectedInstance = ko.observable(hash[0] !== 'null' ? hash[0] : null);
  self.setInstance = function(i) {
    self.selectedInstance(i);
  }

  self.loadWordList = function(cb) {
    if (self.wordList) {
      cb(self.wordList);
    } else {
      $.get('wordlist.txt', function(data) {
        self.wordList = data.split('\n');
        cb(self.wordList)
      });
    }
  }
  function getRandomSubarray(arr, size) {
    // Optimized Fisher-Yates shuffle, from https://stackoverflow.com/a/11935263
    var shuffled = arr.slice(0), i = arr.length, min = i - size, temp, index;
    while (i-- > min) {
      index = Math.floor((i + 1) * Math.random());
      temp = shuffled[index];
      shuffled[index] = shuffled[i];
      shuffled[i] = temp;
    }
    return shuffled.slice(min);
  }

  self.genPSK = function() {
    self.loadWordList(function(words) {
      $('#newconn #psk').val(getRandomSubarray(words, 4).join('-'));
    });
  };

  $("#myTab a").on("shown.bs.tab", function(e) {
		let tid = $(e.target).attr("id");
    self.shownTab(tid.split('-tab')[0]);
    self.refresh();
  });

	let toast = new bootstrap.Toast($("#toast"));
	self.toastData.subscribe(function(v) {
		toast.show();
	});

  self._streamingGet = function(url, req, obs) {
    $.get(url, req, function(data) { 
      let result = [];
      for (let line of data.split('\n')) {
        if (line.trim() === "") {
          continue;
        }
        result.push(JSON.parse(line));
      }
      obs(result);
    });
  }

  self.updatePeerTracking = function() {
    self._streamingGet("/peers/tracking", {instance: self.selectedInstance()}, self.peerTracking);
  };

  self.updatePeerStatuses = function() {
    self._streamingGet("/peers/status", {instance: self.selectedInstance()}, self.peerStatuses);
  }

  self.updateTimeline = function() {
    self._streamingGet("/timeline", {instance: self.selectedInstance()}, self.timeline.update);
  };


  self.randomizeTestClient = function() {
    var arr = new Uint8Array(4);
    window.crypto.getRandomValues(arr);
    dec2hex = function(dec) {
      return dec.toString(16).padStart(2, "0");
    };
    name = Array.from(arr, dec2hex).join('');

    $("#clientstatus input#network").val(name);
    $("#clientstatus input#name").val(name);
    $("#clientstatus input#active_record").val("test");
    $("#clientstatus input#active_unit").val("test");
    $("#clientstatus input#status").val("testing");
    $("#clientstatus input#profile").val("{}");
    $("#clientstatus input#latitude").val((180*Math.random()) - 90);
    $("#clientstatus input#longitude").val((360*Math.random()) - 180);
  }
  self.setStatus = function() {
    var data = $('#clientstatus input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    data.instance = self.selectedInstance();
    $("#clientstatus input").each(function(i,v){v.value = ""});
    $.post("/clients/set_status", data).done((data) => {
			self.showToast("Success", "Status sent");
    }).fail(() => {
			self.showToast("Error", "Failed to set status");
    });
  }

  self.trackPeer = function() {
    $.post("/peers/track", {name: $('#peertrackname').val(), instance: self.selectedInstance()}).done(function(data) {
      self.showToast("Success", "Tracking applied");
    }).fail(() => {
      self.showToast("Error", "Failed to apply tracking");
    });
  };

  self.updateServerSummary = function() {
    $.getJSON("/serverSummary", {instance: self.selectedInstance()}, function(data) { 
      self.serverSummary(data);
    });
  };
  self.updateStorageSummary = function() {
    $.getJSON("/storageSummary", {instance: self.selectedInstance()}, function(data) { 
      self.storageSummary(data);
    });
  };
  self.updateEvents = function() {
    self._streamingGet("/events", {instance: self.selectedInstance()}, self.events);
  };
  self.syncManual = function() {
    $.get("/server/sync", {instance: self.selectedInstance()}, function() {
			self.showToast("Success", "Sync requested; see logs for more details");
      self.updateStorageSummary();
    });
  }
  self.syncLobby = function() {
    $.getJSON("/lobby/sync", {seconds: 60}, function() {
      console.log("lobby sync'd");
      self.updateLobby();
    });
  }
  self.updateLobby = function() {
    self._streamingGet("/lobby", undefined, (data) => {
      self.lobbyStats({local: data[0], world: data[1]});
      self.lobby(data.slice(2));
    });
  };
  self.updateRegistry = function() {
    self._streamingGet("/registry", undefined, self.registry);
  };
  
  
  self.updateInstances = function() {
    self._streamingGet("/connection", undefined, (data) => {
			let names = [];
			for (let c of data) {
				names.push(c.network);
			}
      self.instances(names);
			self.connections(data);
    });
  };

  self.refreshTS = ko.observable(new Date());
  self.refresh = function() {
    self.updateInstances();
    if (!self.selectedInstance()) {
      return;
    }
    switch (self.shownTab()) {
      case "timeline":
        self.updateTimeline();  
        self.updatePeerLogs();
        break;
      case "server":
        self.updateServerSummary();
        break;
      case "storage":
        self.updateStorageSummary();
        break;
      case "events":
        self.updateEvents();
        break;
      case "peers":
        self.updatePeerTracking();
        break;
      case "clients":
        self.updatePeerStatuses();
        break;
      case "lobby":
        self.updateLobby(true);
        break;
      case "registry":
        self.updateRegistry();
    }
    self.refreshTS(new Date());
  };
  self.refresh();
  self.loop = setInterval(self.refresh, 5*1000);

  self.newConnection = function() {
    var data = $('#newconn input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    $("#newconn input").each(function(i,v){v.value = ""});
    $.post("/connection/new", data).done((data) => {
			self.showToast("Success", "Connection created");
			self.gotoTab("conns")
    }).fail(() => {
			self.showToast("Error", "Failed to create connection");
    });
  };

  self.newRegistry = function(v) {
    var data = $('#newregistry input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    $("#newregistry input").each(function(i,v){v.value = ""});
    $.post("/registry/new", data).done((data) => {
			self.showToast("Success", "Network published successfully");
      self.gotoTab("registry");
    }).fail(() => {
			self.showToast("Error", "Failed to publish network");
    });
  };

  self.deleteRegistry = function(v) {
    $.post("/registry/delete", {uuid: v.uuid, local: v.local}).done((data) => {
			self.showToast("Success", "Network published successfully");
      self.gotoTab("registry");
    }).fail(() => {
			self.showToast("Error", "Failed to publish network");
    });
  }

  self.newPassword = function(v) {
    var data = $('#newpassword input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    $.post("/password/new", data).done((data) => {
			self.showToast("Success", "Password changed successfully");
    }).fail(() => {
			self.showToast("Error", "Failed to change password");
    });
  };

	self.gotoPublish = function(v, e) {
		let advform = $("#newregistry");
		advform.find("#name").val(v.network);
		advform.find("#rendezvous").val(v.rendezvous);
		advform.find("#creator").val(v.display_name);
		advform.find("#local").val(v.local);
		self.gotoTab("newregistry", e);
	};
  self.gotoConnect = function(v, e) {
		let advform = $("#newconn");
		advform.find("#name").val(v.network);
		advform.find("#rendezvous").val(v.rendezvous);
		advform.find("#local").val(v.local);
		self.gotoTab("newconn", e);
  };

	self.deleteConn = function(v, e) {
		console.log("deleting", v, e)
    $.post("/connection/delete", {network: v.network}).done((data) => {
			self.showToast("Success", "Deleted connection: " + v.network);
    }).fail(() => {
			self.showToast("Error", "Failed to delete connection");
    });
		e.preventDefault();
	};

  self.href = ko.computed(function() {
    let inst = self.selectedInstance();
    let tab = self.shownTab();
    if (!inst || !tab) {
      return;
    }
    window.location.hash =  `${inst}.${tab}`;
    self.refresh();
  });


  self.getWebAuthnCredentials = function() {
    $.getJSON("/register/credentials", 
        {instance: self.selectedInstance()}, self.credentials);
  };
  self.getWebAuthnCredentials();

  self.removeWebAuthnCredential = function(id) {
    $.post('/register/remove', {id})
      .then((result) => {
        self.showToast("Success", "Removed credential");
        self.getWebAuthnCredentials();
      })
      .catch((err) => {
        self.showToast("Error", "Failed to remove (see console)");
        console.error(err);
      });
  }

	// ArrayBuffer to URLBase64
	function bufferEncode(value) {
		return btoa(String.fromCharCode.apply(null, new Uint8Array(value)))
			.replace(/\+/g, "-")
			.replace(/\//g, "_")
			.replace(/=/g, "");;
	}
  // Base64 to ArrayBuffer
  function bufferDecode(base64url) {
    base64url = base64url.toString();
    base64 =  base64url.replace(/\-/g, "+").replace(/_/g, "/");
    return Uint8Array.from(atob(base64), c => c.charCodeAt(0));
  }

  self.registerUser = function() {
    $.get(
      '/register/begin', null, function (data) { return data }, 'json')
      .then((ccOpts) => {
        console.log(ccOpts);
        ccOpts.publicKey.challenge = bufferDecode(ccOpts.publicKey.challenge);
        ccOpts.publicKey.user.id = bufferDecode(ccOpts.publicKey.user.id);
        return navigator.credentials.create({
          publicKey: ccOpts.publicKey
        })
      })
      .then((credential) => {
        console.log(credential);
        return $.post(
          '/register/finish',
          JSON.stringify({
            id: credential.id,
            rawId: bufferEncode(credential.rawId),
            type: credential.type,
            response: {
              attestationObject: bufferEncode(credential.response.attestationObject),
              clientDataJSON: bufferEncode(credential.response.clientDataJSON),
            },
          }),
          function (data) { return data },
          'json')
      })
      .then((success) => {
			  self.showToast("Success", "Device enrolled successfully");
        self.getWebAuthnCredentials();
      })
      .catch((error) => {
        console.log(error);
			  self.showToast("Error", "Failed enrollment (see console)");
      })
  };
}

