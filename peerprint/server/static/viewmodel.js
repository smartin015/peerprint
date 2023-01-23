
function AppViewModel() {
  let self = this;
  self.history = ko.observableArray([]);
	self.serverSummary = ko.observable();
	self.storageSummary = ko.observable();

	self.updateHistory = function() {
		$.getJSON("/history", function(data) { 
			self.history(data);
		});
	}
	self.updateServerSummary = function() {
		$.getJSON("/serverSummary", function(data) { 
			self.serverSummary(data);
		});
	};
	self.updateStorageSummary = function() {
		$.getJSON("/storageSummary", function(data) { 
			self.storageSummary(data);
		});
	};
	self.updateHistory();
	self.updateServerSummary();
	self.updateStorageSummary();
}
