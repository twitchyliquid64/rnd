
var app = angular.module('kcdb', ['ui.materialize', 'angularMoment']);

app.controller('BodyController', ["$scope", "$rootScope", function ($scope, $rootScope) {
    $scope.page = "home";
    $scope.changePage = function(pageName){
        $scope.page = pageName;
        $rootScope.$broadcast('page-change', {page: pageName});
    };
}]);

app.controller('HomeController', ["$scope", "$http", "$rootScope", "$interval", function ($scope, $http, $rootScope, $interval) {
    $scope.loading = true;
    $scope.status = {};
    $scope.vpns = [];

    $scope.loadStatus = function(){
      $scope.loading = true;
      $scope.error = null;
      $http({
        method: 'GET',
        url: '/status',
      }).then(function successCallback(response) {
        $scope.status = response.data;
        $scope.loading = false;
        $scope.vpn = response.data.config.vpn.name;
        $scope.loadVPNs();
      }, function errorCallback(response) {
        $scope.loading = false;
        $scope.error = response;
      });
    }

    $scope.loadVPNs = function(){
      $scope.error = null;
      $http({
        method: 'GET',
        url: '/vpns',
      }).then(function successCallback(response) {
        $scope.vpns = response.data;
      }, function errorCallback(response) {
        $scope.loading = false;
        $scope.error = response;
      });
    }

    $scope.vpnIcon = function(name){
      for (var i = 0; i < $scope.vpns.length; i++) {
        if ($scope.vpns[i].name == name)
          return $scope.vpns[i].icon;
      }
      return '';
    }

    $scope.vpnChanged = function(){
      if ($scope.ignoreVpnChange) {
        $scope.ignoreVpnChange = false;
        return;
      }

      console.log("VPN changed to: ", $scope.vpn);
      $scope.loading = true;
      $scope.error = null;
      $http({
        method: 'POST',
        url: '/setVPN',
        data: {name: $scope.vpn},
      }).then(function successCallback(response) {
        $scope.loading = false;
      }, function errorCallback(response) {
        $scope.loading = false;
        $scope.error = response;
      });
    }

    // error info helpers.
    $scope.ec = function(){
      if (!$scope.error)return null;
      if ($scope.error.success === false)
        return 'N/A';
      return $scope.error.status;
    }
    $scope.exp = function(){
      if (!$scope.error)return null;
      if ($scope.error.status === -1)
        return "Network Error or server offline";
      if ($scope.error.success === false)
        return 'The server encountered a problem handling the request';
      return $scope.error.statusText;
    }

    $scope.loadStatus();
}]);
