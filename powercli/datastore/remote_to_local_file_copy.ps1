$datastore = Get-Datastore -Name "vsanDatastore"
$sourcePath = $datastore.ExtensionData.Info.Name + ":/VMNAME/VMNAME.vmdk"
$destinationPath = "C:\savehere"
Copy-DatastoreItem -Item $sourcePath -Destination $destinationPath