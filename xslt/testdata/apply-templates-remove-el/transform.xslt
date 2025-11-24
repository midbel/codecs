<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<languages>
			<xsl:apply-templates/>
		</languages>
	</xsl:template>

	<xsl:template match="text()">
		<xsl:value-of select="normalize-space(.)"/>
	</xsl:template>

	<xsl:template match="name">
		<name>
			<xsl:apply-templates/>
		</name>
	</xsl:template>

	<xsl:template match="version">
		<version>
			<xsl:apply-templates/>
		</version>
	</xsl:template>
	
	<xsl:template match="star"/>

	<xsl:template match="language">
		<language>
			<xsl:apply-templates/>
		</language>
	</xsl:template>

	
</xsl:stylesheet>